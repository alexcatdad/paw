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
