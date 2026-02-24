package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/alexcatdad/paw/internal/fsx"
	"github.com/alexcatdad/paw/internal/testutil"
)

func executeCommand(t *testing.T, repoDir string, home string, args ...string) (string, string) {
	t.Helper()
	return executeCommandWithDeps(t, Dependencies{}, repoDir, home, args...)
}

func executeCommandWithDeps(t *testing.T, deps Dependencies, repoDir string, home string, args ...string) (string, string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("PAW_REPO", repoDir)
	cmd := NewRootCommandWithDeps(deps)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command %v failed: %v\nstderr=%s", args, err, stderr.String())
	}
	return stdout.String(), stderr.String()
}

func TestDryRunFlowCommands(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeTestRepo(t, repoDir)

	executeCommand(t, repoDir, home, "--dry-run", "install")
	executeCommand(t, repoDir, home, "--dry-run", "unlink")
	executeCommand(t, repoDir, home, "--dry-run", "sync", "--skip-update")
}

func TestCLIHelpSnapshots(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeTestRepo(t, repoDir)

	tests := []struct {
		name   string
		args   []string
		golden string
	}{
		{name: "root", args: []string{"--help"}, golden: "paw-help.golden"},
		{name: "status", args: []string{"status", "--help"}, golden: "paw-status-help.golden"},
		{name: "doctor", args: []string{"doctor", "--help"}, golden: "paw-doctor-help.golden"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _ := executeCommand(t, repoDir, home, tt.args...)
			assertGolden(t, tt.golden, stdout)
		})
	}
}

func TestAdditionalCommandRunPaths(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeTestRepo(t, repoDir)
	backupFile := filepath.Join(home, ".zshrc.backup.222")
	if err := os.WriteFile(backupFile, []byte("backup"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "dotfiles.config.ts"), []byte("export default {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	revParseCalls := 0
	fake := &testutil.FakeRunner{
		LookPathFn: func(file string) (string, error) {
			switch file {
			case "gh":
				return "gh", nil
			case "paw":
				return filepath.Join(home, "paw"), nil
			default:
				return file, nil
			}
		},
		OutputFn: func(name string, args ...string) ([]byte, error) {
			line := strings.Join(args, " ")
			switch {
			case strings.Contains(line, "release view latest"):
				return []byte(`{"tagName":"v9.9.9"}`), nil
			case strings.Contains(line, "release download --help"):
				return []byte("verify-attestation"), nil
			case strings.Contains(line, "rev-parse HEAD"):
				revParseCalls++
				if revParseCalls == 1 {
					return []byte("abc\n"), nil
				}
				return []byte("def\n"), nil
			case strings.Contains(line, "status --porcelain"):
				return []byte(" M paw.toml\n"), nil
			case strings.Contains(line, "diff --name-only"):
				return []byte("home/.zshrc\n"), nil
			}
			return []byte(""), nil
		},
		CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
			line := strings.Join(args, " ")
			switch {
			case strings.Contains(line, "rev-list --left-right --count"):
				return []byte("0 0"), nil
			case strings.Contains(line, "pull --rebase"):
				return []byte("ok"), nil
			case strings.Contains(line, "status --porcelain"):
				return []byte(" M paw.toml\n"), nil
			case strings.Contains(line, "add -A"):
				return []byte("ok"), nil
			case strings.Contains(line, "commit -m"):
				return []byte("ok"), nil
			case strings.HasSuffix(line, "push"):
				return []byte("ok"), nil
			case strings.HasPrefix(line, "--version"):
				return []byte("paw v9.9.9"), nil
			case strings.HasPrefix(line, "release download latest"):
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

	commands := [][]string{
		{"backup", "list"},
		{"--dry-run", "backup", "clean"},
		{"--dry-run", "backup", "restore", backupFile},
		{"--dry-run", "sync", "--skip-update"},
		{"--dry-run", "init", "https://github.com/example/dotfiles"},
		{"--dry-run", "push", "update dotfiles"},
		{"--dry-run", "update"},
		{"doctor"},
		{"audit", "--json"},
		{"scaffold", "list"},
		{"--dry-run", "migrate-ts-config"},
	}
	for _, args := range commands {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			_, _ = executeCommandWithDeps(t, deps, repoDir, home, args...)
		})
	}

	// roll back needs last-run state to avoid expected "no state" error.
	if err := os.WriteFile(filepath.Join(home, ".zprofile"), []byte("link"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".zprofile.backup.10"), []byte("backup"), 0o644); err != nil {
		t.Fatal(err)
	}
	stateJSON := `{"timestamp":"2026-02-24T00:00:00Z","command":"install","backups":[{"original":"` + filepath.Join(home, ".zprofile") + `","backup":"` + filepath.Join(home, ".zprofile.backup.10") + `","timestamp":10}],"symlinks":[{"source":"src","target":"` + filepath.Join(home, ".zprofile") + `"}]}`
	stateDir := filepath.Join(home, ".config", "paw", "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "last-run.json"), []byte(stateJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = executeCommandWithDeps(t, deps, repoDir, home, "--dry-run", "rollback")
}

func TestCompletionAndManSnapshots(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeTestRepo(t, repoDir)

	out, _ := executeCommand(t, repoDir, home, "completion", "bash")
	assertGolden(t, "paw-completion-bash.golden", out)

	manDir := filepath.Join(t.TempDir(), "man")
	_, _ = executeCommand(t, repoDir, home, "man", "--dir", manDir)
	manPage, err := os.ReadFile(filepath.Join(manDir, "paw.1"))
	if err != nil {
		t.Fatal(err)
	}
	normalized := regexp.MustCompile(`(?m)^\.TH .*`).ReplaceAllString(string(manPage), ".TH PAW 1 <DATE> paw \"paw Manual\"")
	assertGolden(t, "paw-man.golden", normalized)
}

func assertGolden(t *testing.T, name string, actual string) {
	t.Helper()
	path := filepath.Join("..", "..", "tests", "golden", name)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(actual), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	if strings.TrimSpace(string(want)) != strings.TrimSpace(actual) {
		t.Fatalf("golden mismatch for %s", name)
	}
}

func TestRootCommandWithDepsSupportsUnsupportedCompletionShell(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeTestRepo(t, repoDir)
	deps := Dependencies{
		Runner: &testutil.FakeRunner{
			LookPathFn: func(file string) (string, error) { return "", errors.New("unused") },
		},
		FS:    fsx.NewOSFS(),
		Clock: testutil.FakeClock{},
	}
	t.Setenv("HOME", home)
	t.Setenv("PAW_REPO", repoDir)
	cmd := NewRootCommandWithDeps(deps)
	cmd.SetArgs([]string{"completion", "powershell"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected unsupported shell error")
	}
}
