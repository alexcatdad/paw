package symlink

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/output"
)

func TestBuildEntriesWithOverridesAndIgnore(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.MkdirAll(filepath.Join(repoDir, "home"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "home", ".zshrc"), []byte("z"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "home", ".gitconfig"), []byte("g"), 0o644); err != nil {
		t.Fatal(err)
	}
	overrideSource := filepath.Join(repoDir, "templates", "ssh_config")
	if err := os.MkdirAll(filepath.Dir(overrideSource), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overrideSource, []byte("Host *"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.Ignore.Paths = []string{".gitconfig"}
	cfg.Overrides = map[string]config.Override{
		"templates/ssh_config": {
			Target:   "~/.ssh/config",
			Platform: []string{"darwin", "linux"},
		},
	}

	entries, err := BuildEntries(repoDir, home, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		seen[entry.SourceRel] = true
	}
	if !seen["home/.zshrc"] || !seen["templates/ssh_config"] {
		t.Fatalf("unexpected entries: %#v", entries)
	}
}

func TestStatusAndRemove(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	t.Setenv("HOME", home)

	source := filepath.Join(repoDir, "home", ".zshrc")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, ".zshrc")
	if err := os.Symlink(source, target); err != nil {
		t.Fatal(err)
	}

	entries := []Entry{
		{SourceAbs: source, TargetAbs: target},
		{SourceAbs: filepath.Join(repoDir, "home", ".missing"), TargetAbs: filepath.Join(home, ".missing")},
		{SourceAbs: source, TargetAbs: filepath.Join(home, ".conflict"), Platform: []string{"windows"}},
	}
	if err := os.WriteFile(filepath.Join(home, ".conflict"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	states, err := Status(entries)
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 3 {
		t.Fatalf("expected 3 states, got %d", len(states))
	}
	if states[0].Status != StatusLinked && states[1].Status != StatusLinked && states[2].Status != StatusLinked {
		t.Fatalf("expected at least one linked state, got %#v", states)
	}

	logger := output.NewLogger("text", true, false)
	if err := Remove(entries, LinkOptions{}, logger); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("expected linked target removed, err=%v", err)
	}
}

func TestHelpersAndRollbackTransaction(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	source := filepath.Join(home, "src")
	target := filepath.Join(home, "dst")
	if err := os.WriteFile(source, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(source, target); err != nil {
		t.Fatal(err)
	}

	ok, err := isSymlinkTo(target, source)
	if err != nil || !ok {
		t.Fatalf("expected symlink match, ok=%v err=%v", ok, err)
	}
	if !isIgnored(".config/nvim", []string{"~/.config/nvim"}) {
		t.Fatal("expected ignore match")
	}

	created := filepath.Join(home, ".tmp-created")
	if err := os.WriteFile(created, []byte("c"), 0o644); err != nil {
		t.Fatal(err)
	}
	orig := filepath.Join(home, ".orig")
	backup := orig + ".backup.1"
	if err := os.WriteFile(backup, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	logger := output.NewLogger("text", true, false)
	if err := rollbackTransaction(transaction{
		Created: []string{created},
		Backups: []backupMove{{Original: orig, Backup: backup}},
	}, logger); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(orig); err != nil {
		t.Fatalf("expected original restored: %v", err)
	}
}
