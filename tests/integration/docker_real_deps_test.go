package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func requireDockerRealDeps(t *testing.T) {
	t.Helper()
	if os.Getenv("PAW_DOCKER_REAL_DEPS") != "1" {
		t.Skip("set PAW_DOCKER_REAL_DEPS=1 to run real-deps docker tests")
	}
	if runtime.GOOS != "linux" {
		t.Skip("docker real-deps tests run on linux")
	}
}

func TestDockerRealDepsDoctorReflectsActualTools(t *testing.T) {
	requireDockerRealDeps(t)

	home := t.TempDir()
	repoDir := t.TempDir()
	writeRepoFixture(t, repoDir)

	stdout, err := runPawCommandCaptureStdout(t, repoDir, home, "--verbose", "doctor")
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(stdout, "git - NOT FOUND") {
		t.Fatalf("expected git to exist in docker test image, got:\n%s", stdout)
	}
	if _, lookErr := exec.LookPath("zsh"); lookErr != nil {
		if !strings.Contains(stdout, "zsh - NOT FOUND") {
			t.Fatalf("expected missing zsh to be reported, got:\n%s", stdout)
		}
	} else if strings.Contains(stdout, "zsh - NOT FOUND") {
		t.Fatalf("zsh is present but doctor reported missing, got:\n%s", stdout)
	}
}

func TestDockerRealDepsSyncWorksAgainstRealGitRepo(t *testing.T) {
	requireDockerRealDeps(t)

	home := t.TempDir()
	repoDir := t.TempDir()
	writeRepoFixture(t, repoDir)

	if err := exec.Command("git", "-C", repoDir, "init").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", repoDir, "config", "user.name", "paw-test").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", repoDir, "config", "user.email", "paw-test@example.com").Run(); err != nil {
		t.Fatal(err)
	}
	readme := filepath.Join(repoDir, "README.md")
	if err := os.WriteFile(readme, []byte("repo"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", repoDir, "add", "-A").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", repoDir, "commit", "-m", "init").Run(); err != nil {
		t.Fatal(err)
	}

	// No upstream configured is a common real-world state; sync should still return cleanly.
	if err := runPawCommand(t, repoDir, home, "sync", "--skip-update"); err != nil {
		t.Fatal(err)
	}
}

func TestDockerRealDepsDoctorMissingDepsWithPathIsolation(t *testing.T) {
	requireDockerRealDeps(t)

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
		t.Fatalf("expected missing required git in isolated PATH, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Missing optional tools:") || !strings.Contains(stdout, "sh") || !strings.Contains(stdout, "brew") {
		t.Fatalf("expected missing optional tools sh and brew, got:\n%s", stdout)
	}
}
