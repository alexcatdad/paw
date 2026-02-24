package repo

import (
	"errors"
	"os"
	"path/filepath"
)

func HomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", errors.New("could not determine home directory")
	}
	return home, nil
}

func ExpandHome(path string) (string, error) {
	if len(path) >= 2 && path[:2] == "~/" {
		home, err := HomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

func ContractHome(path string) string {
	home, err := HomeDir()
	if err != nil {
		return path
	}
	if path == home {
		return "~"
	}
	prefix := home + string(filepath.Separator)
	if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
		return "~/" + path[len(prefix):]
	}
	return path
}

func RepoDir() (string, error) {
	if env := os.Getenv("PAW_REPO"); env != "" {
		return env, nil
	}
	if env := os.Getenv("DOTFILES_DIR"); env != "" {
		return env, nil
	}
	cwd, err := os.Getwd()
	if err == nil {
		if fileExists(filepath.Join(cwd, "paw.toml")) {
			return cwd, nil
		}
	}
	home, err := HomeDir()
	if err != nil {
		return "", err
	}
	candidates := []string{
		filepath.Join(home, "dotfiles"),
		filepath.Join(home, ".dotfiles"),
		filepath.Join(home, "Projects", "dotfiles"),
		filepath.Join(home, "projects", "dotfiles"),
	}
	for _, candidate := range candidates {
		if fileExists(filepath.Join(candidate, "paw.toml")) {
			return candidate, nil
		}
	}
	return candidates[0], nil
}

func ConfigPath(repoDir string) string {
	return filepath.Join(repoDir, "paw.toml")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func ValidateWithinBase(path string, base string) error {
	cleanPath := filepath.Clean(path)
	cleanBase := filepath.Clean(base)
	if cleanPath == cleanBase {
		return nil
	}
	prefix := cleanBase + string(filepath.Separator)
	if len(cleanPath) >= len(prefix) && cleanPath[:len(prefix)] == prefix {
		return nil
	}
	return errors.New("path escapes allowed base")
}
