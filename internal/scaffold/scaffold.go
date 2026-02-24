package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/repo"
)

var templates = map[string]string{
	"shell config": "home/.zshrc",
	"git config":   "home/.gitconfig",
	"ssh config":   "home/.ssh/config",
	"starship":     "home/.config/starship.toml",
}

var defaultContents = map[string]string{
	"home/.zshrc":                "# Zsh configuration\nsource ~/.zshrc.local 2>/dev/null || true\n",
	"home/.gitconfig":            "[init]\n  defaultBranch = main\n",
	"home/.ssh/config":           "Host *\n  ServerAliveInterval 60\n",
	"home/.config/starship.toml": "format = \"$directory$character\"\n",
}

func List(logger *output.Logger) {
	logger.Header("Available Configs")
	for name, path := range templates {
		logger.Info(fmt.Sprintf("%s -> %s", name, path))
	}
}

func Create(repoDir string, names []string, dryRun bool, force bool, logger *output.Logger) (int, error) {
	created := 0
	for _, name := range names {
		key := strings.ToLower(strings.TrimSpace(name))
		target, ok := templates[key]
		if !ok {
			logger.Warn(fmt.Sprintf("Unknown scaffold target: %s", name))
			continue
		}
		fullPath := filepath.Join(repoDir, filepath.FromSlash(target))
		if err := repo.ValidateWithinBase(fullPath, repoDir); err != nil {
			return created, err
		}
		if _, err := os.Stat(fullPath); err == nil && !force {
			logger.Info(fmt.Sprintf("Skipping existing: %s", target))
			continue
		}
		if dryRun {
			logger.DryRun(fmt.Sprintf("Would create %s", target))
			created++
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return created, err
		}
		content := defaultContents[target]
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			return created, err
		}
		logger.Success(fmt.Sprintf("Created %s", target))
		created++
	}
	return created, nil
}
