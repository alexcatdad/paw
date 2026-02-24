package packages

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
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

func TestEnsureBrewAndLinuxFontErrorBranches(t *testing.T) {
	logger := output.NewLogger("text", true, false)

	t.Run("brew install completes but still missing", func(t *testing.T) {
		missingAfterInstall := &testutil.FakeRunner{
			LookPathFn: func(string) (string, error) { return "", errors.New("missing") },
			RunWithFn:  func(string, []string, execx.CommandOptions) error { return nil },
		}
		withRunner(t, missingAfterInstall)
		if _, err := ensureBrew(false, logger); err == nil {
			t.Fatal("expected ensureBrew missing-after-install error")
		}
	})

	t.Run("tmp dir mkdir error", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		tmpDir := filepath.Join("/tmp", "paw-font-"+strconv.Itoa(os.Getpid()))
		_ = os.RemoveAll(tmpDir)
		if err := os.WriteFile(tmpDir, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Remove(tmpDir) })

		withRunner(t, &testutil.FakeRunner{
			CombinedOutputFn: func(string, ...string) ([]byte, error) { return []byte(""), nil },
		})
		if err := installLinuxFont(false, logger); err == nil {
			t.Fatal("expected tmp mkdir error")
		}
	})

	t.Run("curl error", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		withRunner(t, &testutil.FakeRunner{
			CombinedOutputFn: func(string, ...string) ([]byte, error) { return []byte(""), nil },
			RunFn: func(name string, args ...string) error {
				if name == "curl" {
					return errors.New("curl failed")
				}
				return nil
			},
		})
		if err := installLinuxFont(false, logger); err == nil {
			t.Fatal("expected curl error")
		}
	})

	t.Run("unzip error", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		withRunner(t, &testutil.FakeRunner{
			CombinedOutputFn: func(string, ...string) ([]byte, error) { return []byte(""), nil },
			RunFn: func(name string, args ...string) error {
				if name == "unzip" {
					return errors.New("unzip failed")
				}
				return nil
			},
		})
		if err := installLinuxFont(false, logger); err == nil {
			t.Fatal("expected unzip error")
		}
	})

	t.Run("font dir mkdir error", func(t *testing.T) {
		root := t.TempDir()
		homeFile := filepath.Join(root, "home-file")
		if err := os.WriteFile(homeFile, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HOME", homeFile)
		withRunner(t, &testutil.FakeRunner{
			CombinedOutputFn: func(string, ...string) ([]byte, error) { return []byte(""), nil },
			RunFn:            func(string, ...string) error { return nil },
		})
		if err := installLinuxFont(false, logger); err == nil {
			t.Fatal("expected font dir mkdir error")
		}
	})
}
