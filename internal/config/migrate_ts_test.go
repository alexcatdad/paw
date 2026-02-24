package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateTSConfig(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "dotfiles.config.ts")
	target := filepath.Join(dir, "paw.toml")
	content := `export default {
  symlinks: {
    "shell/zshrc": ".zshrc",
  },
  packages: {
    common: ["ripgrep"],
    darwin: ["ghostty"],
    linux: {
      apt: ["git"],
      brew: ["fd"],
    },
  },
  templates: {
    "templates/zshrc.local.template": ".zshrc.local",
  },
  ignore: [".zshrc.local"],
};`
	if err := os.WriteFile(source, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := MigrateTSConfig(source, target)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Symlinks) != 1 || len(result.Common) != 1 {
		t.Fatalf("unexpected migration result: %+v", result)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatal("expected paw.toml")
	}
}
