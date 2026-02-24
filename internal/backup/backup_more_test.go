package backup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/output"
)

func TestFindAllAndList(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	files := []string{
		filepath.Join(home, ".zshrc.backup.100"),
		filepath.Join(home, ".config", "nvim.backup.200"),
		filepath.Join(home, ".claude", "settings.backup.300"),
	}
	for _, path := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	backups, err := FindAll(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 3 {
		t.Fatalf("expected 3 backups, got %d", len(backups))
	}
	logger := output.NewLogger("text", true, false)
	if err := List(logger); err != nil {
		t.Fatal(err)
	}
}

func TestRollbackRestoresFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	target := filepath.Join(home, ".zshrc")
	backupPath := target + ".backup.111"
	linkTarget := filepath.Join(home, ".gitconfig")

	if err := os.WriteFile(target, []byte("bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backupPath, []byte("good"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(linkTarget, []byte("link"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := SaveLastRun(LastRunState{
		Command: "install",
		Backups: []BackupEntry{{Original: target, Backup: backupPath, Timestamp: 111}},
		Symlinks: []SymlinkEntry{
			{Source: "/tmp/source", Target: linkTarget},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	logger := output.NewLogger("text", true, false)
	if err := Rollback(cfg, Options{SkipHooks: true}, logger); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "good" {
		t.Fatalf("expected restored content, got %q", string(content))
	}
	if _, err := os.Stat(linkTarget); !os.IsNotExist(err) {
		t.Fatalf("expected symlink target removed, err=%v", err)
	}
}
