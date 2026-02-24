package packages

import (
	"testing"

	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/output"
)

func TestInstallAllDryRun(t *testing.T) {
	cfg := config.PackageConfig{
		Common:    []string{"ripgrep"},
		Darwin:    []string{"ghostty"},
		LinuxAPT:  []string{"git"},
		LinuxBrew: []string{"fd"},
	}
	logger := output.NewLogger("text", true, false)
	result := InstallAll(cfg, Options{DryRun: true}, logger)
	if len(result.Installed) == 0 {
		t.Fatal("expected dry-run install list")
	}
}

func TestCheckPackages(t *testing.T) {
	installed, missing := Check(config.PackageConfig{Common: []string{"sh", "command-that-does-not-exist"}})
	if len(installed) == 0 {
		t.Fatal("expected at least one installed command")
	}
	if len(missing) == 0 {
		t.Fatal("expected missing package")
	}
}
