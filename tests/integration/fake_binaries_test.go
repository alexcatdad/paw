package integration_test

import (
	"bytes"
	"io"
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

func runPawCommandCaptureStdout(t *testing.T, repoDir string, home string, args ...string) (string, error) {
	t.Helper()
	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	defer func() {
		os.Stdout = originalStdout
	}()

	runErr := runPawCommand(t, repoDir, home, args...)
	_ = writer.Close()

	content, readErr := io.ReadAll(reader)
	_ = reader.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}

	return string(content), runErr
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

func writeRepoConfig(t *testing.T, repoDir string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repoDir, "paw.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeExecutable(t *testing.T, dir string, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/usr/bin/env sh\nexit 0\n"), 0o755); err != nil {
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

func TestInstallSkipsDarwinBrewPackagesOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-specific package-selection scenario")
	}

	fakes := fakeBinDir(t)
	prependPath(t, fakes)
	logFile := filepath.Join(t.TempDir(), "fake-tools.log")
	t.Setenv("FAKE_TOOL_LOG", logFile)

	home := t.TempDir()
	repoDir := t.TempDir()
	writeRepoFixture(t, repoDir)
	writeRepoConfig(t, repoDir, `version = 1
layout = "hybrid"

[packages]
common = []
darwin = ["ghostty"]
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
`)

	if err := runPawCommand(t, repoDir, home, "--dry-run", "install"); err != nil {
		t.Fatal(err)
	}

	logContent, err := os.ReadFile(logFile)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Fatal(err)
	}
	if strings.Contains(string(logContent), "ghostty") {
		t.Fatalf("darwin package should not be installed on linux, got log:\n%s", string(logContent))
	}
}

func TestLinkWorksWhenShellIsBashButDotfilesContainZshConfigs(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeRepoFixture(t, repoDir)
	t.Setenv("SHELL", "/bin/bash")

	source := filepath.Join(repoDir, "home", ".zshrc")
	if err := os.WriteFile(source, []byte("export PATH=\"$PATH\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runPawCommand(t, repoDir, home, "link"); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(home, ".zshrc")
	linkTarget, err := os.Readlink(target)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(linkTarget) != filepath.Clean(source) {
		t.Fatalf("expected zsh config linked to %s, got %s", source, linkTarget)
	}
}

func TestDoctorReportsMissingRequiredAndOptionalDeps(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeRepoFixture(t, repoDir)
	writeRepoConfig(t, repoDir, `version = 1
layout = "hybrid"

[packages]
common = ["sh", "brew"]
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
`)

	emptyPath := filepath.Join(t.TempDir(), "empty-bin")
	if err := os.MkdirAll(emptyPath, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", emptyPath)

	stdout, err := runPawCommandCaptureStdout(t, repoDir, home, "--verbose", "doctor")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "git - NOT FOUND") {
		t.Fatalf("expected doctor to report missing required git dependency, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Missing optional tools:") || !strings.Contains(stdout, "sh") || !strings.Contains(stdout, "brew") {
		t.Fatalf("expected doctor to report missing optional tools sh and brew, got:\n%s", stdout)
	}
}

func TestDoctorFailsWhenDotfilesRepoConfigIsMisconfigured(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeRepoFixture(t, repoDir)
	writeRepoConfig(t, repoDir, `version = 1
layout = "hybrid"
unexpected_key = true
`)

	err := runPawCommand(t, repoDir, home, "doctor")
	if err == nil {
		t.Fatal("expected doctor to fail for misconfigured paw.toml")
	}
}

func TestInstallUsesLinuxBrewAndNotWSLBrewOnPlainLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-specific package-selection scenario")
	}
	t.Setenv("WSL_DISTRO_NAME", "")
	t.Setenv("WSL_INTEROP", "")

	fakes := fakeBinDir(t)
	prependPath(t, fakes)
	logFile := filepath.Join(t.TempDir(), "fake-tools.log")
	t.Setenv("FAKE_TOOL_LOG", logFile)

	home := t.TempDir()
	repoDir := t.TempDir()
	writeRepoFixture(t, repoDir)
	writeRepoConfig(t, repoDir, `version = 1
layout = "hybrid"

[packages]
common = []
darwin = []
linux_apt = []
linux_brew = ["fd"]
wsl_apt = []
wsl_brew = ["bat"]

[hooks]

[ignore]
paths = []

[backup]
enabled = true
max_age = 30
max_count = 5
`)

	if err := runPawCommand(t, repoDir, home, "install", "--no-interactive"); err != nil {
		t.Fatal(err)
	}

	logContent, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	logText := string(logContent)
	if !strings.Contains(logText, "brew install fd") {
		t.Fatalf("expected linux_brew package to be installed, got:\n%s", logText)
	}
	if strings.Contains(logText, "brew install bat") {
		t.Fatalf("did not expect wsl_brew package on plain linux, got:\n%s", logText)
	}
}

func TestInstallHandlesMissingBrewInNonDryRunGracefully(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeRepoFixture(t, repoDir)

	emptyPath := filepath.Join(t.TempDir(), "empty-bin")
	if err := os.MkdirAll(emptyPath, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", emptyPath)

	stdout, err := runPawCommandCaptureStdout(t, repoDir, home, "install", "--no-interactive")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "Homebrew unavailable:") {
		t.Fatalf("expected warning about missing brew in non-dry-run install, got:\n%s", stdout)
	}
}

func TestDoctorReportsMissingZshWhenShellIsBash(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeRepoFixture(t, repoDir)
	t.Setenv("SHELL", "/bin/bash")

	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"git", "curl", "nano", "ssh", "tar", "gzip"} {
		writeExecutable(t, binDir, tool)
	}
	t.Setenv("PATH", binDir)

	stdout, err := runPawCommandCaptureStdout(t, repoDir, home, "doctor")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "zsh - NOT FOUND") {
		t.Fatalf("expected doctor to flag missing zsh while shell is bash, got:\n%s", stdout)
	}
}

func TestStatusFailsWhenPawTomlMissing(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "home"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := runPawCommand(t, repoDir, home, "status")
	if err == nil {
		t.Fatal("expected status failure when paw.toml is missing")
	}
}
