package drift

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/fsx"
	"github.com/alexcatdad/paw/internal/symlink"
	"github.com/alexcatdad/paw/internal/testutil"
)

func withDeps(t *testing.T, r execx.Runner) {
	t.Helper()
	SetDependencies(r, fsx.NewOSFS(), testutil.FakeClock{Instant: time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)})
	symlink.SetDependencies(r, fsx.NewOSFS(), testutil.FakeClock{Instant: time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)})
}

func TestParseScope(t *testing.T) {
	got, err := ParseScope("")
	if err != nil {
		t.Fatal(err)
	}
	if got != ScopeAll {
		t.Fatalf("expected default scope all, got %q", got)
	}
	got, err = ParseScope("files")
	if err != nil {
		t.Fatal(err)
	}
	if got != ScopeFiles {
		t.Fatalf("expected files scope, got %q", got)
	}
	if _, err := ParseScope("bad"); err == nil {
		t.Fatal("expected invalid scope error")
	}
}

func TestInspectFilesClassifiesManagedDrift(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	withDeps(t, &testutil.FakeRunner{})

	regularSource := filepath.Join(repoDir, "home", ".zshrc")
	if err := os.MkdirAll(filepath.Dir(regularSource), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(regularSource, []byte("repo"), 0o644); err != nil {
		t.Fatal(err)
	}
	regularTarget := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(regularTarget, []byte("local"), 0o644); err != nil {
		t.Fatal(err)
	}

	sourceMissingTarget := filepath.Join(home, ".missing-source")
	if err := os.WriteFile(sourceMissingTarget, []byte("local-only"), 0o644); err != nil {
		t.Fatal(err)
	}

	missingTargetSource := filepath.Join(repoDir, "home", ".gitconfig")
	if err := os.WriteFile(missingTargetSource, []byte("[user]"), 0o644); err != nil {
		t.Fatal(err)
	}

	symlinkTargetSource := filepath.Join(repoDir, "home", ".vimrc")
	if err := os.WriteFile(symlinkTargetSource, []byte("set nocompatible"), 0o644); err != nil {
		t.Fatal(err)
	}
	otherPath := filepath.Join(home, ".other")
	if err := os.WriteFile(otherPath, []byte("other"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(otherPath, filepath.Join(home, ".vimrc")); err != nil {
		t.Fatal(err)
	}

	dirSource := filepath.Join(repoDir, "home", ".config", "test", "config")
	if err := os.MkdirAll(filepath.Dir(dirSource), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dirSource, []byte("repo"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".config", "test", "config"), 0o755); err != nil {
		t.Fatal(err)
	}

	entries := []symlink.Entry{
		{SourceAbs: regularSource, TargetAbs: regularTarget},
		{SourceAbs: filepath.Join(repoDir, "home", ".missing-source"), TargetAbs: sourceMissingTarget},
		{SourceAbs: missingTargetSource, TargetAbs: filepath.Join(home, ".gitconfig")},
		{SourceAbs: symlinkTargetSource, TargetAbs: filepath.Join(home, ".vimrc")},
		{SourceAbs: dirSource, TargetAbs: filepath.Join(home, ".config", "test", "config")},
	}

	findings, err := InspectFiles(Context{Entries: entries})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 5 {
		t.Fatalf("expected 5 findings, got %d", len(findings))
	}

	kinds := map[FindingKind]bool{}
	for _, finding := range findings {
		kinds[finding.Kind] = true
	}
	for _, expected := range []FindingKind{
		KindFileConflict,
		KindFileSourceMissingTarget,
		KindFileMissing,
		KindFileUnsupportedSymlinkTarget,
		KindFileUnsupportedDirTarget,
	} {
		if !kinds[expected] {
			t.Fatalf("missing expected finding kind %s in %#v", expected, findings)
		}
	}
}

func TestInspectBrewStates(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	entries := []symlink.Entry{}
	ctx := Context{Scope: ScopePackages, RepoPath: repoDir, HomePath: home, Entries: entries}
	brewfilePath := filepath.Join(repoDir, "home", ".config", "homebrew", "Brewfile")

	t.Run("unavailable", func(t *testing.T) {
		withDeps(t, &testutil.FakeRunner{
			LookPathFn: func(file string) (string, error) {
				return "", errors.New("missing")
			},
		})
		findings, err := InspectBrew(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(findings) != 1 || findings[0].Kind != KindBrewUnavailable {
			t.Fatalf("unexpected findings: %#v", findings)
		}
	})

	t.Run("missing tracked file", func(t *testing.T) {
		withDeps(t, fakeBrewRunner(t, "brew \"ripgrep\"\n", nil))
		findings, err := InspectBrew(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(findings) != 1 || findings[0].Kind != KindBrewTrackedFileMissing {
			t.Fatalf("unexpected findings: %#v", findings)
		}
	})

	t.Run("drift", func(t *testing.T) {
		withDeps(t, fakeBrewRunner(t, "brew \"fd\"\n", nil))
		if err := os.MkdirAll(filepath.Dir(brewfilePath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(brewfilePath, []byte("brew \"ripgrep\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		findings, err := InspectBrew(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(findings) != 1 || findings[0].Kind != KindBrewfileDrift {
			t.Fatalf("unexpected findings: %#v", findings)
		}
	})

	t.Run("in sync", func(t *testing.T) {
		withDeps(t, fakeBrewRunner(t, "brew \"fd\"\n", nil))
		if err := os.WriteFile(brewfilePath, []byte("brew \"fd\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		findings, err := InspectBrew(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %#v", findings)
		}
	})
}

func TestApplyFilesImportsAndRelinks(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	t.Setenv("HOME", home)
	withDeps(t, &testutil.FakeRunner{})

	source := filepath.Join(repoDir, "home", ".zshrc")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("repo"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(target, []byte("local"), 0o644); err != nil {
		t.Fatal(err)
	}

	missingSource := filepath.Join(repoDir, "home", ".config", "tmux", "tmux.conf")
	missingTarget := filepath.Join(home, ".config", "tmux", "tmux.conf")
	if err := os.MkdirAll(filepath.Dir(missingTarget), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(missingTarget, []byte("set -g mouse on"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := []symlink.Entry{
		{SourceAbs: source, TargetAbs: target},
		{SourceAbs: missingSource, TargetAbs: missingTarget},
	}

	result, err := Apply(Context{
		Scope:    ScopeFiles,
		RepoPath: repoDir,
		HomePath: home,
		Entries:  entries,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Applied) != 2 {
		t.Fatalf("expected 2 applied findings, got %+v", result)
	}
	if len(result.RemainingDrift) != 0 {
		t.Fatalf("expected no remaining drift, got %+v", result.RemainingDrift)
	}
	if len(result.Backups) == 0 {
		t.Fatalf("expected backup entries from relink, got %+v", result)
	}

	got, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "local" {
		t.Fatalf("expected source import to match local file, got %q", string(got))
	}
	got, err = os.ReadFile(missingSource)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "set -g mouse on" {
		t.Fatalf("expected missing source import, got %q", string(got))
	}

	linkDest, err := os.Readlink(target)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(linkDest) != filepath.Clean(source) {
		t.Fatalf("expected target relinked to %s, got %s", source, linkDest)
	}
}

func TestApplyFilesSkipsMissingTargets(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	t.Setenv("HOME", home)
	withDeps(t, &testutil.FakeRunner{})

	source := filepath.Join(repoDir, "home", ".gitconfig")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("[user]"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Apply(Context{
		Scope:    ScopeFiles,
		RepoPath: repoDir,
		HomePath: home,
		Entries: []symlink.Entry{
			{SourceAbs: source, TargetAbs: filepath.Join(home, ".gitconfig")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Applied) != 0 {
		t.Fatalf("expected no applied findings, got %+v", result.Applied)
	}
	if len(result.RemainingDrift) != 1 || result.RemainingDrift[0].Kind != KindFileMissing {
		t.Fatalf("expected missing drift to remain, got %+v", result.RemainingDrift)
	}
}

func TestApplyBrewRewritesTrackedFile(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	withDeps(t, fakeBrewRunner(t, "brew \"ripgrep\"\n", nil))

	brewfilePath := filepath.Join(repoDir, "home", ".config", "homebrew", "Brewfile")
	if err := os.MkdirAll(filepath.Dir(brewfilePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brewfilePath, []byte("brew \"fd\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Apply(Context{
		Scope:    ScopePackages,
		RepoPath: repoDir,
		HomePath: home,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Applied) != 1 {
		t.Fatalf("expected one brew apply result, got %+v", result)
	}
	if len(result.RemainingDrift) != 0 {
		t.Fatalf("expected no remaining brew drift, got %+v", result.RemainingDrift)
	}
	got, err := os.ReadFile(brewfilePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != `brew "ripgrep"` {
		t.Fatalf("expected rewritten Brewfile, got %q", string(got))
	}
}

func fakeBrewRunner(t *testing.T, snapshot string, runErr error) execx.Runner {
	t.Helper()
	return &testutil.FakeRunner{
		LookPathFn: func(file string) (string, error) {
			if file == "brew" {
				return "brew", nil
			}
			return file, nil
		},
		RunFn: func(name string, args ...string) error {
			if runErr != nil {
				return runErr
			}
			if name != "brew" {
				return nil
			}
			for i := 0; i < len(args)-1; i++ {
				if args[i] == "--file" {
					path := args[i+1]
					return os.WriteFile(path, []byte(snapshot), 0o644)
				}
			}
			return nil
		},
	}
}
