package symlink

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alexcatdad/paw/internal/output"
)

func TestCreateLink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repoDir := t.TempDir()
	source := filepath.Join(repoDir, "home", ".zshrc")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, ".zshrc")
	logger := output.NewLogger("text", true, false)
	states, err := Create([]Entry{{SourceRel: "home/.zshrc", SourceAbs: source, TargetRel: ".zshrc", TargetAbs: target}}, LinkOptions{DryRun: false, Force: false, NoInteractive: true}, logger)
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 1 || (states[0].Status != StatusLinked && states[0].Status != StatusBackup) {
		t.Fatalf("unexpected states: %+v", states)
	}
	link, err := os.Readlink(target)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(link) != filepath.Clean(source) {
		t.Fatalf("expected %s, got %s", source, link)
	}
}

func TestConflictWithoutForce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repoDir := t.TempDir()
	source := filepath.Join(repoDir, "home", ".zshrc")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	logger := output.NewLogger("text", true, false)
	states, err := Create([]Entry{{SourceRel: "home/.zshrc", SourceAbs: source, TargetRel: ".zshrc", TargetAbs: target}}, LinkOptions{DryRun: false, Force: false, NoInteractive: true}, logger)
	if err != nil {
		t.Fatal(err)
	}
	if states[0].Status != StatusConflict {
		t.Fatalf("expected conflict, got %s", states[0].Status)
	}
}

func TestConflictWithForceCreatesBackup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repoDir := t.TempDir()
	source := filepath.Join(repoDir, "home", ".zshrc")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	logger := output.NewLogger("text", true, false)
	states, err := Create([]Entry{{SourceRel: "home/.zshrc", SourceAbs: source, TargetRel: ".zshrc", TargetAbs: target}}, LinkOptions{DryRun: false, Force: true, NoInteractive: true}, logger)
	if err != nil {
		t.Fatal(err)
	}
	if states[0].BackupPath == "" {
		t.Fatalf("expected backup path, got %+v", states[0])
	}
	if _, err := os.Stat(states[0].BackupPath); err != nil {
		t.Fatal("expected backup file")
	}
}
