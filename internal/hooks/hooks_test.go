package hooks

import (
	"testing"

	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/output"
)

func TestRunHookDryRun(t *testing.T) {
	cfg := config.Default()
	cfg.Hooks.PreInstall = "echo pre"
	logger := output.NewLogger("text", true, false)
	if err := Run("pre_install", cfg, Options{DryRun: true}, logger); err != nil {
		t.Fatal(err)
	}
}

func TestRunHookCommand(t *testing.T) {
	cfg := config.Default()
	cfg.Hooks.PostInstall = "echo post"
	logger := output.NewLogger("text", true, false)
	if err := Run("post_install", cfg, Options{}, logger); err != nil {
		t.Fatal(err)
	}
}
