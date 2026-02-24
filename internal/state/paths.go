package state

import (
	"os"
	"path/filepath"

	"github.com/alexcatdad/paw/internal/repo"
)

func stateDir() (string, error) {
	home, err := repo.HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "paw", "state"), nil
}

func Ensure() (string, error) {
	dir, err := stateDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func LastRunPath() (string, error) {
	dir, err := Ensure()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "last-run.json"), nil
}

func LegacyLastRunPath() (string, error) {
	home, err := repo.HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".dotfiles-last-run.json"), nil
}

func UpdateStatePath() (string, error) {
	dir, err := Ensure()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "update-state.json"), nil
}

func TransactionPath() (string, error) {
	dir, err := Ensure()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "transaction.json"), nil
}

func LockPath() (string, error) {
	dir, err := Ensure()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lock"), nil
}
