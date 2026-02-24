package backup

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexcatdad/paw/internal/clock"
	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/hooks"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/state"
	"github.com/alexcatdad/paw/internal/testutil"
)

type renameFailFS struct{ testutil.OSBackedFS }

func (renameFailFS) Rename(string, string) error { return errors.New("rename failed") }

func TestRestoreValidationAndDryRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	logger := output.NewLogger("text", true, false)

	if err := Restore(filepath.Join(home, "not-a-backup"), false, logger); err == nil {
		t.Fatal("expected invalid backup file name error")
	}
	if err := Restore(filepath.Join("..", "outside.backup.1"), false, logger); err == nil {
		t.Fatal("expected path validation error")
	}

	backupPath := filepath.Join(home, ".zshrc.backup.123")
	if err := os.WriteFile(backupPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Restore(backupPath, true, logger); err != nil {
		t.Fatal(err)
	}

	SetDependencies(renameFailFS{}, clock.RealClock{})
	t.Cleanup(func() { SetDependencies(testutil.OSBackedFS{}, clock.RealClock{}) })
	if err := Restore(backupPath, false, logger); err == nil {
		t.Fatal("expected restore rename error")
	}
}

func TestCleanDryRunAndClockCutoff(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	logger := output.NewLogger("text", true, false)

	oldBackup := filepath.Join(home, ".zshrc.backup.1000")
	newBackup := filepath.Join(home, ".zshrc.backup.2000")
	if err := os.WriteFile(oldBackup, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newBackup, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	SetDependencies(fsys, testutil.FakeClock{Instant: time.UnixMilli(3000)})
	t.Cleanup(func() { SetDependencies(fsys, clock.RealClock{}) })
	removed, err := Clean(config.BackupConfig{Enabled: true, MaxAge: 0, MaxCount: 1}, true, logger)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 0 {
		t.Fatalf("expected dry-run removed count 0, got %d", removed)
	}
}

func TestRollbackHookErrors(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := config.Default()
	cfg.Hooks.PreRollback = "echo pre"
	logger := output.NewLogger("text", true, false)

	hooks.SetDependencies(&testutil.FakeRunner{
		RunWithFn: func(string, []string, execx.CommandOptions) error {
			return errors.New("hook failed")
		},
	})
	t.Cleanup(func() { hooks.SetDependencies(execx.NewOSRunner()) })

	err := Rollback(cfg, Options{SkipHooks: false}, logger)
	if err == nil || !strings.Contains(err.Error(), "hook pre_rollback failed") {
		t.Fatalf("expected pre hook failure, got %v", err)
	}
}

func TestSaveLoadLastRunEdgePaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	SetDependencies(testutil.FailingFS{}, clock.RealClock{})
	if err := SaveLastRun(LastRunState{Command: "install"}); err == nil {
		t.Fatal("expected save error from failing fs")
	}
	SetDependencies(testutil.OSBackedFS{}, clock.RealClock{})

	currentPath, err := state.LastRunPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(currentPath, []byte("{invalid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	legacyPath, err := state.LegacyLastRunPath()
	if err != nil {
		t.Fatal(err)
	}
	legacy := `{"timestamp":"2026-02-24T00:00:00Z","command":"legacy"}`
	if err := os.WriteFile(legacyPath, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadLastRun()
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil || loaded.Command != "legacy" {
		t.Fatalf("expected fallback to legacy state, got %+v", loaded)
	}
}
