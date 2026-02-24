package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexcatdad/paw/internal/fsx"
	"github.com/alexcatdad/paw/internal/testutil"
)

func executeCommandExpectErr(t *testing.T, deps Dependencies, repoDir string, home string, args ...string) error {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("PAW_REPO", repoDir)
	cmd := NewRootCommandWithDeps(deps)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestSyncBranchCoverage(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeTestRepo(t, repoDir)

	revParseCalls := 0
	fake := &testutil.FakeRunner{
		LookPathFn: func(file string) (string, error) {
			if file == "paw" {
				return "", errors.New("no paw in path")
			}
			return file, nil
		},
		OutputFn: func(name string, args ...string) ([]byte, error) {
			line := strings.Join(args, " ")
			switch {
			case strings.Contains(line, "release view latest"):
				return []byte(`{"tagName":"v9.9.9"}`), nil
			case strings.Contains(line, "rev-parse HEAD"):
				revParseCalls++
				if revParseCalls == 1 {
					return []byte("abc"), nil
				}
				return []byte("def"), nil
			case strings.Contains(line, "status --porcelain"):
				return []byte(""), nil
			case strings.Contains(line, "diff --name-only"):
				return []byte("home/.zshrc\n"), nil
			}
			return []byte(""), nil
		},
		CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
			line := strings.Join(args, " ")
			switch {
			case strings.Contains(line, "rev-list --left-right --count"):
				return []byte("0 2"), nil
			case strings.Contains(line, "pull --rebase"):
				return []byte("ok"), nil
			}
			return []byte("ok"), nil
		},
	}
	deps := Dependencies{
		Runner: fake,
		Clock:  testutil.FakeClock{Instant: time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)},
		FS:     fsx.NewOSFS(),
	}
	executeCommand(t, repoDir, home, "--dry-run", "--auto-update", "sync")
	// rerun with injected deps for behind/update paths
	_, _ = executeCommandWithDeps(t, deps, repoDir, home, "--dry-run", "--auto-update", "sync")
}

func TestSyncPullErrorPath(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeTestRepo(t, repoDir)
	fake := &testutil.FakeRunner{
		CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
			line := strings.Join(args, " ")
			switch {
			case strings.Contains(line, "rev-list --left-right --count"):
				return []byte("0 3"), nil
			case strings.Contains(line, "pull --rebase"):
				return []byte("fatal"), errors.New("pull failed")
			}
			return []byte(""), nil
		},
		OutputFn: func(name string, args ...string) ([]byte, error) {
			if strings.Contains(strings.Join(args, " "), "status --porcelain") {
				return []byte(""), nil
			}
			return []byte("abc"), nil
		},
	}
	deps := Dependencies{Runner: fake, Clock: testutil.FakeClock{}, FS: fsx.NewOSFS()}
	err := executeCommandExpectErr(t, deps, repoDir, home, "sync", "--skip-update")
	if err == nil {
		t.Fatal("expected sync pull error")
	}
}

func TestInitNonDryRunTriggersInstallPath(t *testing.T) {
	home := t.TempDir()
	clonePath := filepath.Join(home, "dotfiles")
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(clonePath, "home"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestRepo(t, clonePath)

	fake := &testutil.FakeRunner{
		OutputFn: func(name string, args ...string) ([]byte, error) {
			if strings.Contains(strings.Join(args, " "), "remote get-url origin") {
				return []byte("https://github.com/example/repo.git"), nil
			}
			return []byte(""), nil
		},
	}
	deps := Dependencies{Runner: fake, Clock: testutil.FakeClock{}, FS: fsx.NewOSFS()}
	_, _ = executeCommandWithDeps(t, deps, clonePath, home, "--skip-packages", "--path", clonePath, "init", "https://github.com/example/repo")
}

func TestScaffoldAndMigrateCommands(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeTestRepo(t, repoDir)

	executeCommand(t, repoDir, home, "scaffold", "shell config")
	executeCommand(t, repoDir, home, "--force", "scaffold", "shell config")

	source := filepath.Join(repoDir, "dotfiles.config.ts")
	outputPath := filepath.Join(repoDir, "paw-migrated.toml")
	content := `export default {
  symlinks: {
    "shell/zshrc": ".zshrc",
  },
  packages: {
    common: ["ripgrep"],
    darwin: ["ghostty"],
    linux: {
      apt: ["git"],
      brew: ["fd"],
    },
  },
  templates: {
    "templates/zshrc.local.template": ".zshrc.local",
  },
  ignore: [".zshrc.local"],
};`
	if err := os.WriteFile(source, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	executeCommand(t, repoDir, home, "migrate-ts-config", "--source", source, "--output", outputPath)
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatal(err)
	}
}

func TestLinkUnlinkAndErrorCommands(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeTestRepo(t, repoDir)

	executeCommand(t, repoDir, home, "link")
	if _, err := os.Lstat(filepath.Join(home, ".zshrc")); err != nil {
		t.Fatalf("expected linked file: %v", err)
	}
	executeCommand(t, repoDir, home, "unlink")
	if _, err := os.Lstat(filepath.Join(home, ".zshrc")); !os.IsNotExist(err) {
		t.Fatalf("expected unlink removal, err=%v", err)
	}

	// Missing backup path should return restore error branch.
	if err := executeCommandExpectErr(t, Dependencies{}, repoDir, home, "backup", "restore", "not-a-backup"); err == nil {
		t.Fatal("expected backup restore error")
	}

	// Run rollback in a clean HOME so no previous last-run state exists.
	homeNoState := t.TempDir()
	if err := executeCommandExpectErr(t, Dependencies{}, repoDir, homeNoState, "rollback", "--skip-hooks"); err == nil {
		t.Fatal("expected rollback no state error")
	}
	if err := executeCommandExpectErr(t, Dependencies{}, repoDir, home, "migrate-ts-config", "--source", filepath.Join(repoDir, "missing.ts"), "--output", filepath.Join(repoDir, "out.toml")); err == nil {
		t.Fatal("expected migrate missing source error")
	}
}
