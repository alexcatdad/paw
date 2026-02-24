package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexcatdad/paw/internal/backup"
	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/platform"
	"github.com/alexcatdad/paw/internal/repo"
	"github.com/alexcatdad/paw/internal/symlink"
)

func loadConfigEntries(logger *output.Logger) (string, string, config.Config, []symlink.Entry, error) {
	repoDir, err := repo.RepoDir()
	if err != nil {
		return "", "", config.Config{}, nil, err
	}
	cfgPath := repo.ConfigPath(repoDir)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return "", "", config.Config{}, nil, err
	}
	homeDir, err := repo.HomeDir()
	if err != nil {
		return "", "", config.Config{}, nil, err
	}
	entries, err := symlink.BuildEntries(repoDir, homeDir, cfg)
	if err != nil {
		return "", "", config.Config{}, nil, err
	}
	logger.Debug(fmt.Sprintf("Loaded %d link entries", len(entries)))
	return repoDir, homeDir, cfg, entries, nil
}

func saveLastRun(command string, states []symlink.State) error {
	backups := []backup.BackupEntry{}
	for _, st := range states {
		if strings.TrimSpace(st.BackupPath) != "" {
			backups = append(backups, backup.BackupEntry{Original: st.Target, Backup: st.BackupPath, Timestamp: time.Now().UnixMilli()})
		}
	}
	last := backup.LastRunState{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Command:   command,
		Backups:   backups,
		Symlinks:  backup.StatesToEntries(states),
	}
	return backup.SaveLastRun(last)
}

func generateTemplates(repoDir string, cfg config.Config, dryRun bool, logger *output.Logger) error {
	homeDir, err := repo.HomeDir()
	if err != nil {
		return err
	}
	for source, target := range cfg.Templates {
		sourceAbs := filepath.Join(repoDir, filepath.FromSlash(source))
		targetAbs, err := repo.ExpandHome(target)
		if err != nil {
			return err
		}
		if !filepath.IsAbs(targetAbs) {
			targetAbs = filepath.Join(homeDir, filepath.FromSlash(targetAbs))
		}
		if err := repo.ValidateWithinBase(targetAbs, homeDir); err != nil {
			return err
		}
		if _, err := os.Stat(targetAbs); err == nil {
			continue
		}
		if dryRun {
			logger.DryRun(fmt.Sprintf("Would create template %s", repo.ContractHome(targetAbs)))
			continue
		}
		data, err := os.ReadFile(sourceAbs)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(targetAbs, data, 0o644); err != nil {
			return err
		}
		logger.Success(fmt.Sprintf("Created template %s", repo.ContractHome(targetAbs)))
	}
	return nil
}

func printStatusStates(states []symlink.State, logger *output.Logger) {
	for _, st := range states {
		icon := "○"
		switch st.Status {
		case symlink.StatusLinked:
			icon = "✓"
		case symlink.StatusConflict:
			icon = "⚠"
		case symlink.StatusSourceMissing:
			icon = "✗"
		case symlink.StatusSkipped:
			icon = "-"
		}
		msg := fmt.Sprintf("%s %s (%s)", icon, repo.ContractHome(st.Target), st.Status)
		if st.Reason != "" {
			msg += ": " + st.Reason
		}
		logger.Info(msg)
	}
}

func printSystemTable(logger *output.Logger, repoDir string, homeDir string) {
	logger.Table(map[string]string{
		"System": platform.SystemInfo(),
		"Home":   homeDir,
		"Repo":   repoDir,
	})
}
