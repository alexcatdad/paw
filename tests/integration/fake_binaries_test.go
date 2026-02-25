package integration_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alexcatdad/paw/internal/cli"
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

func runPawCommand(t *testing.T, repoDir string, home string, args ...string) error {
	t.Helper()
	t.Setenv("PAW_REPO", repoDir)
	t.Setenv("HOME", home)
	cmd := cli.NewRootCommand()
	cmd.SetArgs(args)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	return cmd.Execute()
}

func writeRepoFixture(t *testing.T, repoDir string) {
	t.Helper()
	content := `version = 1
layout = "hybrid"

[packages]
common = []
darwin = []
linux_apt = []
linux_brew = []
wsl_apt = []
wsl_brew = []

[hooks]

[ignore]
paths = []

[backup]
enabled = true
max_age = 30
max_count = 5
`
	if err := os.WriteFile(filepath.Join(repoDir, "paw.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "home"), 0o755); err != nil {
		t.Fatal(err)
	}
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

func TestDriftApplyFilesImportsAndRelinks(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeRepoFixture(t, repoDir)

	source := filepath.Join(repoDir, "home", ".zshrc")
	if err := os.WriteFile(source, []byte("repo"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(target, []byte("local-change"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runPawCommand(t, repoDir, home, "drift", "apply", "--scope", "files"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != "local-change" {
		t.Fatalf("expected source imported from target, got %q", string(got))
	}
	linkTarget, err := os.Readlink(target)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(linkTarget) != filepath.Clean(source) {
		t.Fatalf("expected relink to %s, got %s", source, linkTarget)
	}
}

func TestDriftApplyPackagesRewritesBrewfile(t *testing.T) {
	fakes := fakeBinDir(t)
	prependPath(t, fakes)
	t.Setenv("FAKE_BREW_DUMP_CONTENT", "brew \"fd\"\n")

	home := t.TempDir()
	repoDir := t.TempDir()
	writeRepoFixture(t, repoDir)
	tracked := filepath.Join(repoDir, "home", ".config", "homebrew", "Brewfile")
	if err := os.MkdirAll(filepath.Dir(tracked), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tracked, []byte("brew \"ripgrep\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runPawCommand(t, repoDir, home, "drift", "apply", "--scope", "packages"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(tracked)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != `brew "fd"` {
		t.Fatalf("expected rewritten Brewfile, got %q", string(got))
	}
}
