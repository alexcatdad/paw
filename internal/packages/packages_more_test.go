package packages

import (
	"errors"
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
	path, err := ensureBrew(false, logger)
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
	dryPath, err := ensureBrew(true, logger)
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
	installedPath, err := ensureBrew(false, logger)
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
	if _, err := ensureBrew(false, logger); err == nil {
		t.Fatal("expected ensureBrew install error")
	}
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

func TestInstallLinuxFontAndPathJoin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	logger := output.NewLogger("text", true, false)

	alreadyInstalled := &testutil.FakeRunner{
		CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
			return []byte("FiraCode Nerd Font"), nil
		},
	}
	withRunner(t, alreadyInstalled)
	if err := installLinuxFont(false, logger); err != nil {
		t.Fatal(err)
	}

	dryRun := &testutil.FakeRunner{
		CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
			return []byte(""), nil
		},
	}
	withRunner(t, dryRun)
	if err := installLinuxFont(true, logger); err != nil {
		t.Fatal(err)
	}

	calls := []string{}
	installFlow := &testutil.FakeRunner{
		CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
			calls = append(calls, name+" "+strings.Join(args, " "))
			if name == "fc-list" {
				return []byte(""), nil
			}
			return []byte("ok"), nil
		},
		RunFn: func(name string, args ...string) error {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return nil
		},
	}
	withRunner(t, installFlow)
	if err := installLinuxFont(false, logger); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(calls, "\n")
	for _, token := range []string{"curl -fsSL", "unzip -q", "fc-cache -f"} {
		if !strings.Contains(joined, token) {
			t.Fatalf("expected %q in font install calls: %s", token, joined)
		}
	}

	if got := filepathJoin("/tmp", "a", "b"); got != "/tmp/a/b" {
		t.Fatalf("unexpected filepathJoin: %s", got)
	}
	if got := filepathJoin(); got != "" {
		t.Fatalf("expected empty filepathJoin, got %q", got)
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
