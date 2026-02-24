package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestRepo(t *testing.T, root string) {
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
	if err := os.WriteFile(filepath.Join(root, "paw.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "home"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "home", ".zshrc"), []byte("# test"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRootStatus(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeTestRepo(t, repoDir)
	executeCommand(t, repoDir, home, "status")
}

func TestRootLinkDryRun(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	writeTestRepo(t, repoDir)
	executeCommand(t, repoDir, home, "--dry-run", "link")
}

func TestNewRootCommandWrapper(t *testing.T) {
	cmd := NewRootCommand()
	cmd.SetArgs([]string{"--version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}
