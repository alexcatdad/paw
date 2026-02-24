package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "paw.toml")
	content := `version = 1
layout = "hybrid"

[packages]
common = ["ripgrep"]
darwin = []
linux_apt = []
linux_brew = []
wsl_apt = []
wsl_brew = []

[hooks]

[ignore]
paths = [".zshrc.local"]

[backup]
enabled = true
max_age = 30
max_count = 5
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Version != 1 || cfg.Layout != "hybrid" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "paw.toml")
	content := `version = 1
layout = "hybrid"
unknown = true

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
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected unknown key error")
	}
}

func TestValidateRequiresVersionOne(t *testing.T) {
	cfg := Default()
	cfg.Version = 2
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error")
	}
}
