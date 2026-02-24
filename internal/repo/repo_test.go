package repo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alexcatdad/paw/internal/output"
)

func TestNormalizeGitURL(t *testing.T) {
	if normalizeGitURL("git@github.com:user/repo.git") != "github.com/user/repo" {
		t.Fatal("ssh normalize failed")
	}
	if normalizeGitURL("https://github.com/user/repo.git") != "github.com/user/repo" {
		t.Fatal("https normalize failed")
	}
}

func TestValidateWithinBase(t *testing.T) {
	base := "/tmp/base"
	if err := ValidateWithinBase("/tmp/base/a", base); err != nil {
		t.Fatal(err)
	}
	if err := ValidateWithinBase("/tmp/other", base); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestRunInitDryRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	logger := output.NewLogger("text", true, false)
	ok, err := RunInit("https://github.com/example/repo", InitOptions{DryRun: true, Path: filepath.Join(home, "dotfiles")}, logger)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected success")
	}
}

func TestPawConfigSaveLoad(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := PawConfig{DotfilesRepo: "~/dotfiles", RepoURL: "https://github.com/example/repo"}
	if err := SavePawConfig(cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadPawConfig()
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil || loaded.RepoURL != cfg.RepoURL {
		t.Fatalf("unexpected config: %+v", loaded)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "paw", "config.json")); err != nil {
		t.Fatal(err)
	}
}
