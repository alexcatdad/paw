package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/symlink"
)

func TestSaveLastRunAndStatusPrinting(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	logger := output.NewLogger("text", true, false)
	states := []symlink.State{
		{Source: "/tmp/a", Target: filepath.Join(home, ".zshrc"), Status: symlink.StatusLinked},
		{Source: "/tmp/b", Target: filepath.Join(home, ".gitconfig"), Status: symlink.StatusBackup, BackupPath: filepath.Join(home, ".gitconfig.backup.1")},
		{Source: "/tmp/c", Target: filepath.Join(home, ".missing"), Status: symlink.StatusSourceMissing},
	}
	if err := saveLastRun("install", states); err != nil {
		t.Fatal(err)
	}
	printStatusStates(states, logger)
	printSystemTable(logger, "/tmp/repo", home)
}

func TestGenerateTemplates(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	t.Setenv("HOME", home)
	source := filepath.Join(repoDir, "templates", "gitconfig")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("[user]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Templates = map[string]string{
		"templates/gitconfig": "~/.gitconfig",
	}
	logger := output.NewLogger("text", true, false)

	if err := generateTemplates(repoDir, cfg, true, logger); err != nil {
		t.Fatal(err)
	}
	if err := generateTemplates(repoDir, cfg, false, logger); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, ".gitconfig")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected generated template: %v", err)
	}
}
