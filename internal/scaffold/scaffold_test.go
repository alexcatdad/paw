package scaffold

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alexcatdad/paw/internal/output"
)

func TestScaffoldCreateAndList(t *testing.T) {
	repoDir := t.TempDir()
	logger := output.NewLogger("text", true, false)
	List(logger)
	created, err := Create(repoDir, []string{"shell config", "git config"}, false, false, logger)
	if err != nil {
		t.Fatal(err)
	}
	if created == 0 {
		t.Fatal("expected files created")
	}
	if _, err := os.Stat(filepath.Join(repoDir, "home", ".zshrc")); err != nil {
		t.Fatal(err)
	}
}

func TestScaffoldCreateBranches(t *testing.T) {
	repoDir := t.TempDir()
	logger := output.NewLogger("text", true, false)

	created, err := Create(repoDir, []string{"unknown", "shell config"}, true, false, logger)
	if err != nil {
		t.Fatal(err)
	}
	if created != 1 {
		t.Fatalf("expected one dry-run created count, got %d", created)
	}

	if err := os.MkdirAll(filepath.Join(repoDir, "home"), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(repoDir, "home", ".zshrc")
	if err := os.WriteFile(existing, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	created, err = Create(repoDir, []string{"shell config"}, false, false, logger)
	if err != nil {
		t.Fatal(err)
	}
	if created != 0 {
		t.Fatalf("expected skip existing without force, got %d", created)
	}

	created, err = Create(repoDir, []string{"shell config"}, false, true, logger)
	if err != nil {
		t.Fatal(err)
	}
	if created != 1 {
		t.Fatalf("expected force create count 1, got %d", created)
	}
}
