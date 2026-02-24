package repo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandContractAndRepoDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	expanded, err := ExpandHome("~/dotfiles")
	if err != nil {
		t.Fatal(err)
	}
	if expanded != filepath.Join(home, "dotfiles") {
		t.Fatalf("unexpected expand result: %s", expanded)
	}
	if ContractHome(filepath.Join(home, ".zshrc")) != "~/.zshrc" {
		t.Fatal("contract home failed")
	}

	envRepo := filepath.Join(home, "mydot")
	t.Setenv("PAW_REPO", envRepo)
	repoDir, err := RepoDir()
	if err != nil {
		t.Fatal(err)
	}
	if repoDir != envRepo {
		t.Fatalf("expected env repo dir %s, got %s", envRepo, repoDir)
	}
}

func TestRepoDirFromCwdAndFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PAW_REPO", "")
	t.Setenv("DOTFILES_DIR", "")

	cwdRepo := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwdRepo, "paw.toml"), []byte("version=1\nlayout='hybrid'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	defer func() { _ = os.Chdir(wd) }()
	if err := os.Chdir(cwdRepo); err != nil {
		t.Fatal(err)
	}
	got, err := RepoDir()
	if err != nil {
		t.Fatal(err)
	}
	gotEval, _ := filepath.EvalSymlinks(got)
	repoEval, _ := filepath.EvalSymlinks(cwdRepo)
	if gotEval != repoEval {
		t.Fatalf("expected cwd repo %s, got %s", repoEval, gotEval)
	}

	if err := os.Remove(filepath.Join(cwdRepo, "paw.toml")); err != nil {
		t.Fatal(err)
	}
	got, err = RepoDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(home, "dotfiles") {
		t.Fatalf("expected fallback, got %s", got)
	}
}

func TestConfigPathAndFileExists(t *testing.T) {
	tmp := t.TempDir()
	configPath := ConfigPath(tmp)
	if configPath != filepath.Join(tmp, "paw.toml") {
		t.Fatalf("unexpected config path: %s", configPath)
	}
	if fileExists(configPath) {
		t.Fatal("expected false for missing file")
	}
	if err := os.WriteFile(configPath, []byte("version=1\nlayout='hybrid'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !fileExists(configPath) {
		t.Fatal("expected existing file")
	}
}
