package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alexcatdad/paw/internal/app"
	"github.com/alexcatdad/paw/internal/drift"
	"github.com/alexcatdad/paw/internal/fsx"
	"github.com/alexcatdad/paw/internal/testutil"
)

func executeCommandRaw(t *testing.T, deps Dependencies, repoDir string, home string, args ...string) (string, string, error) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("PAW_REPO", repoDir)
	cmd := NewRootCommandWithDeps(deps)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func TestDriftCommandHelp(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeTestRepo(t, repoDir)

	out, _, err := executeCommandRaw(t, Dependencies{}, repoDir, home, "drift", "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains([]byte(out), []byte("status")) || !bytes.Contains([]byte(out), []byte("apply")) {
		t.Fatalf("expected drift help to include subcommands, got:\n%s", out)
	}
}

func TestDriftStatusExitCode(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeTestRepo(t, repoDir)

	_, _, err := executeCommandRaw(t, Dependencies{}, repoDir, home, "link")
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = executeCommandRaw(t, Dependencies{}, repoDir, home, "drift", "status", "--scope", "files")
	if err != nil {
		t.Fatalf("expected no drift after link, got %v", err)
	}

	target := filepath.Join(home, ".zshrc")
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("local change"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err = executeCommandRaw(t, Dependencies{}, repoDir, home, "drift", "status", "--scope", "files")
	if err == nil {
		t.Fatal("expected conflict exit code when drift exists")
	}
	if app.ExitCode(err) != app.ExitConflict {
		t.Fatalf("expected exit conflict code, got %d (%v)", app.ExitCode(err), err)
	}
}

func TestDriftApplyDryRunAndApply(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeTestRepo(t, repoDir)

	target := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(target, []byte("local version"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(repoDir, "home", ".zshrc")
	before, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = executeCommandRaw(t, Dependencies{}, repoDir, home, "--dry-run", "drift", "apply", "--scope", "files")
	if err == nil {
		t.Fatal("expected dry-run apply to return conflict due remaining drift")
	}
	if app.ExitCode(err) != app.ExitConflict {
		t.Fatalf("expected exit conflict code, got %d", app.ExitCode(err))
	}
	afterDryRun, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterDryRun) != string(before) {
		t.Fatalf("expected dry-run to keep source unchanged, got %q", string(afterDryRun))
	}

	_, _, err = executeCommandRaw(t, Dependencies{}, repoDir, home, "drift", "apply", "--scope", "files")
	if err != nil {
		t.Fatal(err)
	}
	afterApply, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterApply) != "local version" {
		t.Fatalf("expected apply to import local file, got %q", string(afterApply))
	}
}

func TestDriftStatusJSONScopePackages(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeTestRepo(t, repoDir)
	brewfilePath := filepath.Join(repoDir, "home", ".config", "homebrew", "Brewfile")
	if err := os.MkdirAll(filepath.Dir(brewfilePath), 0o755); err != nil {
		t.Fatal(err)
	}
	const snapshot = "brew \"ripgrep\"\n"
	if err := os.WriteFile(brewfilePath, []byte(snapshot), 0o644); err != nil {
		t.Fatal(err)
	}

	fake := &testutil.FakeRunner{
		LookPathFn: func(file string) (string, error) {
			return file, nil
		},
		RunFn: func(name string, args ...string) error {
			for i := 0; i < len(args)-1; i++ {
				if args[i] == "--file" {
					return os.WriteFile(args[i+1], []byte(snapshot), 0o644)
				}
			}
			return nil
		},
	}
	deps := Dependencies{
		Runner: fake,
		Clock:  testutil.FakeClock{Instant: time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)},
		FS:     fsx.NewOSFS(),
	}

	stdout, _, err := executeCommandRaw(t, deps, repoDir, home, "--json", "drift", "status", "--scope", "packages")
	if err != nil {
		t.Fatal(err)
	}
	var report drift.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("expected valid report json, got %v\n%s", err, stdout)
	}
	if report.Scope != drift.ScopePackages {
		t.Fatalf("expected scope packages, got %s", report.Scope)
	}
	if report.Summary.DriftCount != 0 {
		t.Fatalf("expected no package drift, got %+v", report.Summary)
	}
}
