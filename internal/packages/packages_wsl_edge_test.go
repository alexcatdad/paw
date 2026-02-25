package packages

import (
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/testutil"
)

func TestInstallAllWSLBranches(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("WSL detection only applies on linux runtime")
	}
	t.Setenv("WSL_DISTRO_NAME", "Ubuntu")
	t.Setenv("WSL_INTEROP", "")

	logger := output.NewLogger("text", true, false)
	fake := &testutil.FakeRunner{
		LookPathFn: func(file string) (string, error) {
			if file == "brew" {
				return "", errors.New("missing")
			}
			return file, nil
		},
		RunFn: func(name string, args ...string) error {
			switch name {
			case "dpkg":
				return errors.New("missing")
			case "sudo":
				return errors.New("apt failed")
			default:
				return nil
			}
		},
		RunWithFn: func(string, []string, execx.CommandOptions) error {
			return nil
		},
	}
	withRunner(t, fake)

	cfg := config.PackageConfig{
		WSLAPT:  []string{"git"},
		WSLBrew: []string{"bat"},
	}
	result := InstallAll(cfg, Options{DryRun: false}, logger)
	if len(result.Failed) == 0 {
		t.Fatalf("expected failed packages, got %+v", result)
	}
}

func TestInstallAllWSLBrewSelectionAndCheckBranches(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("WSL selection paths only apply on linux runtime")
	}
	t.Setenv("WSL_DISTRO_NAME", "Ubuntu")
	t.Setenv("WSL_INTEROP", "")

	logger := output.NewLogger("text", true, false)
	fake := &testutil.FakeRunner{
		LookPathFn: func(file string) (string, error) {
			if file == "brew" {
				return "/home/linuxbrew/.linuxbrew/bin/brew", nil
			}
			return file, nil
		},
		RunFn: func(name string, args ...string) error {
			line := strings.Join(args, " ")
			if strings.Contains(line, "list --formula") || strings.Contains(line, "list --cask") {
				return errors.New("not installed")
			}
			if line == "install bat" {
				return errors.New("formula missing")
			}
			if strings.Contains(line, "install --cask") {
				return errors.New("still missing")
			}
			return nil
		},
	}
	withRunner(t, fake)

	cfgWithWSLBrew := config.PackageConfig{
		Common:    []string{"ripgrep"},
		WSLBrew:   []string{"bat"},
		LinuxBrew: []string{"fd"},
	}
	result := InstallAll(cfgWithWSLBrew, Options{DryRun: false}, logger)
	if len(result.Installed) == 0 || len(result.Failed) == 0 {
		t.Fatalf("expected mixed install result, got %+v", result)
	}

	installedA, missingA := Check(cfgWithWSLBrew)
	if len(installedA)+len(missingA) == 0 {
		t.Fatal("expected check results for WSL brew list")
	}

	cfgFallback := config.PackageConfig{
		WSLBrew:   []string{},
		LinuxBrew: []string{"sh"},
	}
	installedB, _ := Check(cfgFallback)
	if len(installedB) == 0 {
		t.Fatal("expected linux_brew fallback in WSL check")
	}
}

func TestEnsureBrewAndFontErrorBranches(t *testing.T) {
	logger := output.NewLogger("text", true, false)

	t.Run("brew install completes but still missing", func(t *testing.T) {
		missingAfterInstall := &testutil.FakeRunner{
			LookPathFn: func(string) (string, error) { return "", errors.New("missing") },
			RunWithFn:  func(string, []string, execx.CommandOptions) error { return nil },
		}
		withRunner(t, missingAfterInstall)
		// Confirm with "y" so we reach the brew script, which succeeds but brew
		// is still not found afterward.
		if _, err := ensureBrew(false, strings.NewReader("y\n"), logger); err == nil {
			t.Fatal("expected ensureBrew missing-after-install error")
		}
	})

	t.Run("installFont no brew path", func(t *testing.T) {
		fake := &testutil.FakeRunner{
			RunFn: func(name string, args ...string) error { return errors.New("not installed") },
		}
		withRunner(t, fake)
		if err := installFont(config.PackageConfig{}, "", false, logger); err == nil {
			t.Fatal("expected error when brew path is empty")
		}
	})

	t.Run("installFont brew cask install fails", func(t *testing.T) {
		fake := &testutil.FakeRunner{
			RunFn: func(name string, args ...string) error { return errors.New("brew error") },
		}
		withRunner(t, fake)
		if err := installFont(config.PackageConfig{}, "/brew", false, logger); err == nil {
			t.Fatal("expected error on brew cask install failure")
		}
	})

	t.Run("installFont dry run always succeeds", func(t *testing.T) {
		fake := &testutil.FakeRunner{
			RunFn: func(name string, args ...string) error { return errors.New("not installed") },
		}
		withRunner(t, fake)
		if err := installFont(config.PackageConfig{NerdFont: "Hack"}, "/brew", true, logger); err != nil {
			t.Fatalf("expected dry-run success, got: %v", err)
		}
	})
}
