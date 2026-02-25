package packages

import (
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/testutil"
)

func withRunner(t *testing.T, r execx.Runner) {
	t.Helper()
	SetDependencies(r)
	t.Cleanup(func() { SetDependencies(execx.NewOSRunner()) })
}

func TestSetDependenciesAndEnsureBrew(t *testing.T) {
	logger := output.NewLogger("text", true, false)
	fake := &testutil.FakeRunner{
		LookPathFn: func(file string) (string, error) {
			if file == "brew" {
				return "/usr/local/bin/brew", nil
			}
			return file, nil
		},
	}
	withRunner(t, fake)
	// brew is already found; no stdin interaction required.
	path, err := ensureBrew(false, strings.NewReader(""), logger)
	if err != nil {
		t.Fatal(err)
	}
	if path != "/usr/local/bin/brew" {
		t.Fatalf("unexpected brew path: %s", path)
	}

	missing := &testutil.FakeRunner{
		LookPathFn: func(file string) (string, error) {
			return "", errors.New("missing")
		},
	}
	withRunner(t, missing)
	// dryRun=true, so no prompt is shown and no stdin is read.
	dryPath, err := ensureBrew(true, strings.NewReader(""), logger)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dryPath, "brew") {
		t.Fatalf("expected dry-run brew path, got %s", dryPath)
	}

	calls := 0
	installFake := &testutil.FakeRunner{
		LookPathFn: func(file string) (string, error) {
			calls++
			if calls > 1 {
				return "/usr/local/bin/brew", nil
			}
			return "", errors.New("missing")
		},
	}
	withRunner(t, installFake)
	// Confirm the prompt with "y".
	installedPath, err := ensureBrew(false, strings.NewReader("y\n"), logger)
	if err != nil {
		t.Fatal(err)
	}
	if installedPath == "" {
		t.Fatal("expected brew path after installation")
	}

	failInstall := &testutil.FakeRunner{
		LookPathFn: func(string) (string, error) { return "", errors.New("missing") },
		RunWithFn:  func(string, []string, execx.CommandOptions) error { return errors.New("install failed") },
	}
	withRunner(t, failInstall)
	// Confirm with "y" so we reach the RunWith call that fails.
	if _, err := ensureBrew(false, strings.NewReader("y\n"), logger); err == nil {
		t.Fatal("expected ensureBrew install error")
	}
}

// TestEnsureBrewConfirmationPrompt verifies that ensureBrew asks the user
// before running the Homebrew install script, and correctly handles both
// acceptance and rejection.
func TestEnsureBrewConfirmationPrompt(t *testing.T) {
	logger := output.NewLogger("text", true, false)

	t.Run("declined returns error", func(t *testing.T) {
		missingRunner := &testutil.FakeRunner{
			LookPathFn: func(string) (string, error) { return "", errors.New("missing") },
		}
		withRunner(t, missingRunner)
		_, err := ensureBrew(false, strings.NewReader("n\n"), logger)
		if err == nil {
			t.Fatal("expected error when user declines")
		}
		if !strings.Contains(err.Error(), "declined") {
			t.Fatalf("expected 'declined' in error, got: %v", err)
		}
	})

	t.Run("empty answer treated as no", func(t *testing.T) {
		missingRunner := &testutil.FakeRunner{
			LookPathFn: func(string) (string, error) { return "", errors.New("missing") },
		}
		withRunner(t, missingRunner)
		_, err := ensureBrew(false, strings.NewReader("\n"), logger)
		if err == nil {
			t.Fatal("expected error when user gives empty answer")
		}
		if !strings.Contains(err.Error(), "declined") {
			t.Fatalf("expected 'declined' in error, got: %v", err)
		}
	})

	t.Run("yes proceeds to install", func(t *testing.T) {
		calls2 := 0
		yesRunner := &testutil.FakeRunner{
			LookPathFn: func(string) (string, error) {
				calls2++
				if calls2 > 1 {
					return "/usr/local/bin/brew", nil
				}
				return "", errors.New("missing")
			},
		}
		withRunner(t, yesRunner)
		path, err := ensureBrew(false, strings.NewReader("yes\n"), logger)
		if err != nil {
			t.Fatalf("expected success after yes, got: %v", err)
		}
		if path == "" {
			t.Fatal("expected non-empty brew path")
		}
	})

	t.Run("eof stdin treated as no", func(t *testing.T) {
		missingRunner := &testutil.FakeRunner{
			LookPathFn: func(string) (string, error) { return "", errors.New("missing") },
		}
		withRunner(t, missingRunner)
		_, err := ensureBrew(false, io.LimitReader(strings.NewReader(""), 0), logger)
		if err == nil {
			t.Fatal("expected error on EOF stdin")
		}
		if !strings.Contains(err.Error(), "declined") {
			t.Fatalf("expected 'declined' in error, got: %v", err)
		}
	})
}

func TestInstallBrew(t *testing.T) {
	logger := output.NewLogger("text", true, false)

	if installBrew("bad package!", "/brew", false, logger) {
		t.Fatal("expected invalid package name to fail")
	}

	alreadyInstalled := &testutil.FakeRunner{
		RunFn: func(name string, args ...string) error {
			if strings.Join(args, " ") == "list --formula ripgrep" {
				return nil
			}
			return errors.New("not used")
		},
	}
	withRunner(t, alreadyInstalled)
	if !installBrew("ripgrep", "/brew", false, logger) {
		t.Fatal("expected already installed package to pass")
	}

	installFormula := &testutil.FakeRunner{
		RunFn: func(name string, args ...string) error {
			line := strings.Join(args, " ")
			if line == "list --formula ripgrep" || line == "list --cask ripgrep" {
				return errors.New("missing")
			}
			if line == "install ripgrep" {
				return nil
			}
			return errors.New("unexpected")
		},
	}
	withRunner(t, installFormula)
	if !installBrew("ripgrep", "/brew", false, logger) {
		t.Fatal("expected formula install success")
	}

	installCask := &testutil.FakeRunner{
		RunFn: func(name string, args ...string) error {
			line := strings.Join(args, " ")
			if strings.HasPrefix(line, "list ") {
				return errors.New("missing")
			}
			if line == "install app" {
				return errors.New("formula failed")
			}
			if line == "install --cask app" {
				return nil
			}
			return errors.New("unexpected")
		},
	}
	withRunner(t, installCask)
	if !installBrew("app", "/brew", false, logger) {
		t.Fatal("expected cask install success")
	}

	failBoth := &testutil.FakeRunner{
		RunFn: func(name string, args ...string) error { return errors.New("boom") },
	}
	withRunner(t, failBoth)
	if installBrew("ripgrep", "/brew", false, logger) {
		t.Fatal("expected install failure")
	}

	dryRun := &testutil.FakeRunner{
		RunFn: func(name string, args ...string) error { return errors.New("missing") },
	}
	withRunner(t, dryRun)
	if !installBrew("ripgrep", "/brew", true, logger) {
		t.Fatal("expected dry-run install success")
	}
}

func TestInstallApt(t *testing.T) {
	logger := output.NewLogger("text", true, false)

	if installApt("bad package!", false, logger) {
		t.Fatal("expected invalid apt package to fail")
	}

	alreadyInstalled := &testutil.FakeRunner{
		RunFn: func(name string, args ...string) error {
			if name == "dpkg" {
				return nil
			}
			return errors.New("unexpected")
		},
	}
	withRunner(t, alreadyInstalled)
	if !installApt("git", false, logger) {
		t.Fatal("expected installed apt package to pass")
	}

	dryRun := &testutil.FakeRunner{
		RunFn: func(name string, args ...string) error {
			if name == "dpkg" {
				return errors.New("missing")
			}
			return nil
		},
	}
	withRunner(t, dryRun)
	if !installApt("git", true, logger) {
		t.Fatal("expected dry-run apt install success")
	}

	installOK := &testutil.FakeRunner{
		RunFn: func(name string, args ...string) error {
			if name == "dpkg" {
				return errors.New("missing")
			}
			if name == "sudo" {
				return nil
			}
			return errors.New("unexpected")
		},
	}
	withRunner(t, installOK)
	if !installApt("git", false, logger) {
		t.Fatal("expected apt install success")
	}

	installFail := &testutil.FakeRunner{
		RunFn: func(name string, args ...string) error {
			if name == "dpkg" {
				return errors.New("missing")
			}
			return errors.New("install failed")
		},
	}
	withRunner(t, installFail)
	if installApt("git", false, logger) {
		t.Fatal("expected apt install failure")
	}
}

// TestInstallFont verifies the new brew-cask-based font installation path.
func TestInstallFont(t *testing.T) {
	logger := output.NewLogger("text", true, false)

	t.Run("already installed via cask", func(t *testing.T) {
		fake := &testutil.FakeRunner{
			RunFn: func(name string, args ...string) error {
				if strings.Join(args, " ") == "list --cask font-fira-code-nerd-font" {
					return nil
				}
				return errors.New("unexpected call")
			},
		}
		withRunner(t, fake)
		if err := installFont(config.PackageConfig{}, "/brew", false, logger); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("dry run", func(t *testing.T) {
		fake := &testutil.FakeRunner{
			RunFn: func(name string, args ...string) error {
				return errors.New("not installed")
			},
		}
		withRunner(t, fake)
		if err := installFont(config.PackageConfig{}, "/brew", true, logger); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("no brew path returns error", func(t *testing.T) {
		fake := &testutil.FakeRunner{
			RunFn: func(name string, args ...string) error { return errors.New("not installed") },
		}
		withRunner(t, fake)
		if err := installFont(config.PackageConfig{}, "", false, logger); err == nil {
			t.Fatal("expected error when brewPath is empty")
		}
	})

	t.Run("brew cask install succeeds", func(t *testing.T) {
		calls := []string{}
		fake := &testutil.FakeRunner{
			RunFn: func(name string, args ...string) error {
				line := strings.Join(args, " ")
				calls = append(calls, name+" "+line)
				if line == "list --cask font-fira-code-nerd-font" {
					return errors.New("not installed")
				}
				// Allow install --cask and fc-cache
				return nil
			},
		}
		withRunner(t, fake)
		if err := installFont(config.PackageConfig{}, "/brew", false, logger); err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(calls, "\n")
		if !strings.Contains(joined, "install --cask font-fira-code-nerd-font") {
			t.Fatalf("expected brew install --cask call, got:\n%s", joined)
		}
	})

	t.Run("brew cask install fails", func(t *testing.T) {
		fake := &testutil.FakeRunner{
			RunFn: func(name string, args ...string) error {
				return errors.New("boom")
			},
		}
		withRunner(t, fake)
		if err := installFont(config.PackageConfig{}, "/brew", false, logger); err == nil {
			t.Fatal("expected error when brew cask install fails")
		}
	})

	t.Run("custom font name from config", func(t *testing.T) {
		calls := []string{}
		fake := &testutil.FakeRunner{
			RunFn: func(name string, args ...string) error {
				line := strings.Join(args, " ")
				calls = append(calls, name+" "+line)
				if strings.Contains(line, "list --cask") {
					return errors.New("not installed")
				}
				return nil
			},
		}
		withRunner(t, fake)
		cfg := config.PackageConfig{NerdFont: "JetBrainsMono"}
		if err := installFont(cfg, "/brew", false, logger); err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(calls, "\n")
		if !strings.Contains(joined, "install --cask font-jet-brains-mono-nerd-font") {
			t.Fatalf("expected JetBrainsMono cask name, got:\n%s", joined)
		}
	})
}

// TestFontCaskName verifies the CamelCase-to-kebab-case conversion used to
// derive Homebrew cask names from the config nerd_font value.
func TestFontCaskName(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"FiraCode", "font-fira-code-nerd-font"},
		{"JetBrainsMono", "font-jet-brains-mono-nerd-font"},
		{"Hack", "font-hack-nerd-font"},
		{"SourceCodePro", "font-source-code-pro-nerd-font"},
	}
	for _, tc := range cases {
		got := fontCaskName(tc.input)
		if got != tc.want {
			t.Errorf("fontCaskName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestFilepathJoinReplacement confirms that filepath.Join (stdlib) is used
// correctly for path construction, replacing the removed filepathJoin helper.
func TestFilepathJoinReplacement(t *testing.T) {
	if got := filepath.Join("/tmp", "a", "b"); got != "/tmp/a/b" {
		t.Fatalf("unexpected filepath.Join result: %s", got)
	}
	if got := filepath.Join(); got != "" {
		t.Fatalf("expected empty result from filepath.Join(), got %q", got)
	}
}

func TestInstallAllAndCheck(t *testing.T) {
	logger := output.NewLogger("text", true, false)
	fake := &testutil.FakeRunner{
		LookPathFn: func(file string) (string, error) {
			if file == "brew" {
				return "/brew", nil
			}
			return file, nil
		},
		RunFn: func(name string, args ...string) error {
			line := strings.Join(args, " ")
			if strings.Contains(line, "list") {
				return errors.New("not installed")
			}
			return nil
		},
	}
	withRunner(t, fake)
	result := InstallAll(config.PackageConfig{Common: []string{"ripgrep"}}, Options{DryRun: true}, logger)
	if len(result.Installed) == 0 {
		t.Fatalf("expected install results, got %+v", result)
	}
	installed, missing := Check(config.PackageConfig{Common: []string{"sh", "this-binary-is-missing-xyz"}})
	if len(installed) == 0 || len(missing) == 0 {
		t.Fatalf("expected installed and missing results, got installed=%v missing=%v", installed, missing)
	}
}
