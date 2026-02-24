package repo

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type PawConfig struct {
	DotfilesRepo string `json:"dotfilesRepo"`
	RepoURL      string `json:"repoUrl"`
}

func configDir() (string, error) {
	home, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "paw"), nil
}

func ConfigFilePath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func LoadPawConfig() (*PawConfig, error) {
	path, err := ConfigFilePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var cfg PawConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func SavePawConfig(cfg PawConfig) error {
	path, err := ConfigFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}

func DotfilesPath() (string, error) {
	cfg, err := LoadPawConfig()
	if err != nil {
		return "", err
	}
	if cfg != nil && strings.TrimSpace(cfg.DotfilesRepo) != "" {
		return ExpandHome(cfg.DotfilesRepo)
	}
	return RepoDir()
}
