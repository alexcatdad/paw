package integration_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/packages"
	"github.com/alexcatdad/paw/internal/repo"
	"github.com/alexcatdad/paw/internal/update"
)

func fakeBinDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test path")
	}
	return filepath.Join(filepath.Dir(thisFile), "fakes")
}

func prependPath(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestRepoPullUsesFakeGitBinary(t *testing.T) {
	fakes := fakeBinDir(t)
	prependPath(t, fakes)
	logFile := filepath.Join(t.TempDir(), "fake-tools.log")
	headState := filepath.Join(t.TempDir(), "head-state")
	t.Setenv("FAKE_TOOL_LOG", logFile)
	t.Setenv("FAKE_GIT_STATUS", " M paw.toml")
	t.Setenv("FAKE_GIT_DIFF", "home/.zshrc\nREADME.md")
	t.Setenv("FAKE_GIT_HEAD_STATE", headState)

	changed, err := repo.PullRepo(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) == 0 {
		t.Fatalf("expected changed files, got %#v", changed)
	}
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "git") {
		t.Fatalf("expected git invocations in log, got:\n%s", string(content))
	}
}

func TestUpdateDryRunUsesFakeGhAndPaw(t *testing.T) {
	fakes := fakeBinDir(t)
	tmp := t.TempDir()
	pawBinary := filepath.Join(tmp, "paw")
	if err := os.WriteFile(pawBinary, []byte("#!/usr/bin/env bash\necho paw v9.9.9\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	prependPath(t, tmp)
	prependPath(t, fakes)
	t.Setenv("HOME", t.TempDir())

	logger := output.NewLogger("text", true, false)
	if err := update.Perform("1.0.0", update.Options{DryRun: true, ForceCheck: true}, logger); err != nil {
		t.Fatal(err)
	}
}

func TestPackagesInstallUsesFakeBinaries(t *testing.T) {
	fakes := fakeBinDir(t)
	prependPath(t, fakes)
	t.Setenv("FAKE_BREW_ALREADY_INSTALLED", "")

	cfg := config.PackageConfig{
		Common: []string{"ripgrep"},
	}
	logger := output.NewLogger("text", true, false)
	result := packages.InstallAll(cfg, packages.Options{DryRun: true}, logger)
	if len(result.Installed) == 0 {
		t.Fatalf("expected dry-run install to mark package installed, got %+v", result)
	}
}
