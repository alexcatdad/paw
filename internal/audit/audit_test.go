package audit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/output"
)

func TestRunAudit(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoDir, "paw.toml"), []byte("version = 1\nlayout = \"hybrid\"\n\n[packages]\ncommon=[]\ndarwin=[]\nlinux_apt=[]\nlinux_brew=[]\nwsl_apt=[]\nwsl_brew=[]\n\n[hooks]\n\n[ignore]\npaths=[]\n\n[backup]\nenabled=true\nmax_age=30\nmax_count=5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "home"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	res, err := Run(repoDir, &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Score <= 0 {
		t.Fatalf("unexpected score: %d", res.Score)
	}
	logger := output.NewLogger("text", true, false)
	Print(res, Options{Verbose: true}, logger)
	Print(res, Options{JSON: true}, logger)
}
