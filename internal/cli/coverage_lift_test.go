package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/fsx"
	"github.com/alexcatdad/paw/internal/testutil"
)

type lstatErrorFS struct{ testutil.OSBackedFS }

func (lstatErrorFS) Lstat(string) (os.FileInfo, error) {
	return nil, errors.New("forced lstat failure")
}

type failLastRunWriteFS struct{ testutil.OSBackedFS }

func (failLastRunWriteFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	if strings.HasSuffix(name, "last-run.json") {
		return errors.New("forced last-run write failure")
	}
	return os.WriteFile(name, data, perm)
}

func writePawConfig(t *testing.T, repoDir string, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repoDir, "paw.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func basePawConfig(extra string) string {
	return `version = 1
layout = "hybrid"
` + extra
}

func TestCommandLoadFailures(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeTestRepo(t, repoDir)
	writePawConfig(t, repoDir, "version =\n")

	commands := [][]string{
		{"install"},
		{"link"},
		{"unlink"},
		{"status"},
		{"backup", "clean"},
		{"rollback"},
		{"--skip-update", "sync"},
		{"push", "msg"},
		{"doctor"},
		{"audit"},
	}
	for _, args := range commands {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			if err := executeCommandExpectErr(t, Dependencies{}, repoDir, home, args...); err == nil {
				t.Fatalf("expected error for command %v", args)
			}
		})
	}
}

func TestInstallConflictMissingAndSaveWarningBranches(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeTestRepo(t, repoDir)
	writePawConfig(t, repoDir, basePawConfig(`
[overrides."home/.missing-source"]
target = "~/.missing-target"
`))

	if err := os.WriteFile(filepath.Join(home, ".zshrc"), []byte("conflict"), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := Dependencies{FS: failLastRunWriteFS{}}
	_, _ = executeCommandWithDeps(t, deps, repoDir, home, "--skip-packages", "--no-interactive", "install")
}

func TestStatusVerboseAndLastRunTable(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeTestRepo(t, repoDir)
	writePawConfig(t, repoDir, basePawConfig(`
[packages]
common = ["definitely-missing-bin-xyz"]
darwin = []
linux_apt = []
linux_brew = []
wsl_apt = []
wsl_brew = []
`))

	stateDir := filepath.Join(home, ".config", "paw", "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stateJSON := `{"timestamp":"2026-02-24T00:00:00Z","command":"install","backups":[],"symlinks":[]}`
	if err := os.WriteFile(filepath.Join(stateDir, "last-run.json"), []byte(stateJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _ = executeCommand(t, repoDir, home, "--verbose", "status")
}

func TestDoctorAllChecksPassedAndVerboseMissingOptional(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeTestRepo(t, repoDir)
	writePawConfig(t, repoDir, basePawConfig(`
[packages]
common = ["definitely-missing-bin-xyz"]
darwin = []
linux_apt = []
linux_brew = []
wsl_apt = []
wsl_brew = []
`))

	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"git", "zsh", "curl", "nano", "ssh", "tar", "gzip"} {
		path := filepath.Join(binDir, tool)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	_, _ = executeCommand(t, repoDir, home, "--verbose", "doctor")
}

func TestHookFailureBranches(t *testing.T) {
	t.Run("pre-install", func(t *testing.T) {
		home := t.TempDir()
		repoDir := t.TempDir()
		writeTestRepo(t, repoDir)
		writePawConfig(t, repoDir, basePawConfig(`
[hooks]
pre_install = "false"
`))
		if err := executeCommandExpectErr(t, Dependencies{}, repoDir, home, "--skip-packages", "install"); err == nil {
			t.Fatal("expected pre_install hook failure")
		}
	})

	t.Run("post-install", func(t *testing.T) {
		home := t.TempDir()
		repoDir := t.TempDir()
		writeTestRepo(t, repoDir)
		writePawConfig(t, repoDir, basePawConfig(`
[hooks]
post_install = "false"
`))
		if err := executeCommandExpectErr(t, Dependencies{}, repoDir, home, "--skip-packages", "install"); err == nil {
			t.Fatal("expected post_install hook failure")
		}
	})

	t.Run("pre-link", func(t *testing.T) {
		home := t.TempDir()
		repoDir := t.TempDir()
		writeTestRepo(t, repoDir)
		writePawConfig(t, repoDir, basePawConfig(`
[hooks]
pre_link = "false"
`))
		if err := executeCommandExpectErr(t, Dependencies{}, repoDir, home, "link"); err == nil {
			t.Fatal("expected pre_link hook failure")
		}
	})

	t.Run("post-link", func(t *testing.T) {
		home := t.TempDir()
		repoDir := t.TempDir()
		writeTestRepo(t, repoDir)
		writePawConfig(t, repoDir, basePawConfig(`
[hooks]
post_link = "false"
`))
		if err := executeCommandExpectErr(t, Dependencies{}, repoDir, home, "link"); err == nil {
			t.Fatal("expected post_link hook failure")
		}
	})

	t.Run("pre-sync", func(t *testing.T) {
		home := t.TempDir()
		repoDir := t.TempDir()
		writeTestRepo(t, repoDir)
		writePawConfig(t, repoDir, basePawConfig(`
[hooks]
pre_sync = "false"
`))
		if err := executeCommandExpectErr(t, Dependencies{}, repoDir, home, "--skip-update", "sync"); err == nil {
			t.Fatal("expected pre_sync hook failure")
		}
	})

	t.Run("pre-push", func(t *testing.T) {
		home := t.TempDir()
		repoDir := t.TempDir()
		writeTestRepo(t, repoDir)
		writePawConfig(t, repoDir, basePawConfig(`
[hooks]
pre_push = "false"
`))
		if err := executeCommandExpectErr(t, Dependencies{}, repoDir, home, "push", "msg"); err == nil {
			t.Fatal("expected pre_push hook failure")
		}
	})
}

func TestPushPostHookFailureWithInjectedRunner(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeTestRepo(t, repoDir)
	writePawConfig(t, repoDir, basePawConfig(`
[hooks]
post_push = "false"
`))

	fake := &testutil.FakeRunner{
		CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
			line := strings.Join(args, " ")
			switch {
			case strings.Contains(line, "status --porcelain"):
				return []byte(" M paw.toml\n"), nil
			case strings.Contains(line, "add -A"):
				return []byte("ok"), nil
			case strings.Contains(line, "commit -m"):
				return []byte("ok"), nil
			case strings.HasSuffix(line, "push"):
				return []byte("ok"), nil
			default:
				return []byte("ok"), nil
			}
		},
		RunWithFn: func(name string, args []string, _ execx.CommandOptions) error {
			if name == "sh" && len(args) >= 2 && args[1] == "false" {
				return errors.New("hook failed")
			}
			return nil
		},
	}
	deps := Dependencies{Runner: fake, Clock: testutil.FakeClock{Instant: time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)}, FS: fsx.NewOSFS()}
	if err := executeCommandExpectErr(t, deps, repoDir, home, "push", "msg"); err == nil {
		t.Fatal("expected post_push hook failure")
	}
}

func TestTemplateAndTargetValidationErrorBranches(t *testing.T) {
	t.Run("link-generate-template-error", func(t *testing.T) {
		home := t.TempDir()
		repoDir := t.TempDir()
		writeTestRepo(t, repoDir)
		writePawConfig(t, repoDir, basePawConfig(`
[templates]
"templates/missing.tmpl" = "~/.missing-template"
`))
		if err := executeCommandExpectErr(t, Dependencies{}, repoDir, home, "link"); err == nil {
			t.Fatal("expected template read error")
		}
	})

	t.Run("install-symlink-validate-error", func(t *testing.T) {
		home := t.TempDir()
		repoDir := t.TempDir()
		writeTestRepo(t, repoDir)
		writePawConfig(t, repoDir, basePawConfig(`
[overrides."home/.zshrc"]
target = "/tmp/outside-home-target"
`))
		if err := executeCommandExpectErr(t, Dependencies{}, repoDir, home, "--skip-packages", "install"); err == nil {
			t.Fatal("expected install symlink validation error")
		}
	})

	t.Run("link-symlink-validate-error", func(t *testing.T) {
		home := t.TempDir()
		repoDir := t.TempDir()
		writeTestRepo(t, repoDir)
		writePawConfig(t, repoDir, basePawConfig(`
[overrides."home/.zshrc"]
target = "/tmp/outside-home-target"
`))
		if err := executeCommandExpectErr(t, Dependencies{}, repoDir, home, "link"); err == nil {
			t.Fatal("expected link symlink validation error")
		}
	})

	t.Run("unlink-remove-error", func(t *testing.T) {
		home := t.TempDir()
		repoDir := t.TempDir()
		writeTestRepo(t, repoDir)
		writePawConfig(t, repoDir, basePawConfig(`
[overrides."home/.zshrc"]
target = "/tmp/outside-home-target"
`))
		if err := executeCommandExpectErr(t, Dependencies{}, repoDir, home, "unlink"); err == nil {
			t.Fatal("expected unlink validation error")
		}
	})
}

func TestSyncCreateErrorAfterPull(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeTestRepo(t, repoDir)
	writePawConfig(t, repoDir, basePawConfig(`
[overrides."home/.zshrc"]
target = "/tmp/outside-home-target"
`))

	rev := 0
	fake := &testutil.FakeRunner{
		OutputFn: func(name string, args ...string) ([]byte, error) {
			line := strings.Join(args, " ")
			switch {
			case strings.Contains(line, "status --porcelain"):
				return []byte(""), nil
			case strings.Contains(line, "rev-parse HEAD"):
				rev++
				if rev == 1 {
					return []byte("abc"), nil
				}
				return []byte("def"), nil
			case strings.Contains(line, "diff --name-only"):
				return []byte("home/.zshrc\n"), nil
			default:
				return []byte(""), nil
			}
		},
		CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
			line := strings.Join(args, " ")
			if strings.Contains(line, "rev-list --left-right --count") {
				return []byte("0 1"), nil
			}
			if strings.Contains(line, "pull --rebase") {
				return []byte("ok"), nil
			}
			return []byte("ok"), nil
		},
	}
	deps := Dependencies{Runner: fake, Clock: testutil.FakeClock{Instant: time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)}, FS: fsx.NewOSFS()}
	if err := executeCommandExpectErr(t, deps, repoDir, home, "--skip-update", "sync"); err == nil {
		t.Fatal("expected sync symlink creation error")
	}
}

func TestStatusAndDoctorStatusErrorWithInjectedFS(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeTestRepo(t, repoDir)
	deps := Dependencies{
		Runner: &testutil.FakeRunner{},
		Clock:  testutil.FakeClock{},
		FS:     lstatErrorFS{},
	}
	if err := executeCommandExpectErr(t, deps, repoDir, home, "status"); err == nil {
		t.Fatal("expected status error")
	}
	if err := executeCommandExpectErr(t, deps, repoDir, home, "doctor"); err == nil {
		t.Fatal("expected doctor status error")
	}
}
