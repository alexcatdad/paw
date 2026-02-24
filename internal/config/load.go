package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

func Default() Config {
	return Config{
		Version:   1,
		Layout:    "hybrid",
		Overrides: map[string]Override{},
		Templates: map[string]string{},
		Ignore:    IgnoreConfig{Paths: []string{}},
		Backup: BackupConfig{
			Enabled:  true,
			MaxAge:   30,
			MaxCount: 5,
		},
		Packages: PackageConfig{
			Common:    []string{},
			Darwin:    []string{},
			LinuxAPT:  []string{},
			LinuxBrew: []string{},
			WSLAPT:    []string{},
			WSLBrew:   []string{},
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if strings.TrimSpace(path) == "" {
		return cfg, errors.New("config path is required")
	}
	if _, err := os.Stat(path); err != nil {
		return cfg, fmt.Errorf("configuration file not found: %s", path)
	}
	md, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return cfg, err
	}
	if und := md.Undecoded(); len(und) > 0 {
		parts := make([]string, 0, len(und))
		for _, item := range und {
			parts = append(parts, item.String())
		}
		return cfg, fmt.Errorf("unknown config keys: %s", strings.Join(parts, ", "))
	}
	if err := Validate(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func Validate(cfg Config) error {
	if cfg.Version != 1 {
		return fmt.Errorf("unsupported config version %d (expected 1)", cfg.Version)
	}
	if cfg.Layout != "hybrid" {
		return fmt.Errorf("layout must be 'hybrid', got %q", cfg.Layout)
	}
	for source, override := range cfg.Overrides {
		if strings.TrimSpace(source) == "" {
			return errors.New("override source key cannot be empty")
		}
		if strings.TrimSpace(override.Target) == "" {
			return fmt.Errorf("override %q is missing target", source)
		}
		if filepath.IsAbs(source) {
			return fmt.Errorf("override source %q must be relative", source)
		}
	}
	return nil
}
