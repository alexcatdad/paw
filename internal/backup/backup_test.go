package backup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/output"
)

func TestSaveLoadLastRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	state := LastRunState{Command: "install", Timestamp: "2026-01-01T00:00:00Z"}
	if err := SaveLastRun(state); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadLastRun()
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil || loaded.Command != state.Command {
		t.Fatalf("unexpected loaded state: %+v", loaded)
	}
}

func TestRestoreAndClean(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	orig := filepath.Join(home, ".zshrc")
	backup := orig + ".backup.123"
	if err := os.WriteFile(backup, []byte("backup"), 0o644); err != nil {
		t.Fatal(err)
	}
	logger := output.NewLogger("text", true, false)
	if err := Restore(backup, false, logger); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(orig); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(orig+".backup.100", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	removed, err := Clean(config.BackupConfig{Enabled: true, MaxAge: 0, MaxCount: 0}, false, logger)
	if err != nil {
		t.Fatal(err)
	}
	if removed == 0 {
		t.Fatal("expected removed backups")
	}
}

func TestRollbackNoState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := config.Default()
	logger := output.NewLogger("text", true, false)
	if err := Rollback(cfg, Options{DryRun: false, SkipHooks: true}, logger); err == nil {
		t.Fatal("expected error")
	}
}
