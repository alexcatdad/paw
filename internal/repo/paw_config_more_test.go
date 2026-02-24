package repo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepoDirDotfilesEnvAndDotfilesPathFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	envPath := filepath.Join(home, "dotfiles-env")
	t.Setenv("PAW_REPO", "")
	t.Setenv("DOTFILES_DIR", envPath)
	got, err := RepoDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != envPath {
		t.Fatalf("expected DOTFILES_DIR value, got %s", got)
	}

	t.Setenv("DOTFILES_DIR", "")
	if path, err := ConfigFilePath(); err != nil {
		t.Fatal(err)
	} else {
		_ = os.Remove(path)
	}
	dotfilesPath, err := DotfilesPath()
	if err != nil {
		t.Fatal(err)
	}
	if dotfilesPath == "" {
		t.Fatal("expected dotfiles path fallback")
	}
}

func TestLoadPawConfigInvalidJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path, err := ConfigFilePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPawConfig(); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func TestSavePawConfigErrorWhenHomeIsFile(t *testing.T) {
	fileHome := filepath.Join(t.TempDir(), "home-file")
	if err := os.WriteFile(fileHome, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", fileHome)
	if err := SavePawConfig(PawConfig{DotfilesRepo: "~/dotfiles"}); err == nil {
		t.Fatal("expected save error when HOME is not a directory")
	}
}
