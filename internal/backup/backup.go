package backup

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/hooks"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/repo"
	"github.com/alexcatdad/paw/internal/state"
)

var backupPattern = regexp.MustCompile(`^(.+)\.backup\.(\d+)$`)

type BackupEntry struct {
	Original  string `json:"original"`
	Backup    string `json:"backup"`
	Timestamp int64  `json:"timestamp"`
}

type SymlinkEntry struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type LastRunState struct {
	Timestamp string         `json:"timestamp"`
	Command   string         `json:"command"`
	Backups   []BackupEntry  `json:"backups"`
	Symlinks  []SymlinkEntry `json:"symlinks"`
}

type Options struct {
	DryRun    bool
	SkipHooks bool
}

func SaveLastRun(st LastRunState) error {
	path, err := state.LastRunPath()
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return fsys.WriteFile(path, payload, 0o644)
}

func LoadLastRun() (*LastRunState, error) {
	paths := []func() (string, error){state.LastRunPath, state.LegacyLastRunPath}
	for _, fn := range paths {
		path, err := fn()
		if err != nil {
			continue
		}
		data, err := fsys.ReadFile(path)
		if err != nil {
			continue
		}
		var st LastRunState
		if err := json.Unmarshal(data, &st); err != nil {
			continue
		}
		return &st, nil
	}
	return nil, nil
}

func FindAll(homeDir string) ([]BackupEntry, error) {
	dirs := []string{
		homeDir,
		filepath.Join(homeDir, ".config"),
		filepath.Join(homeDir, ".claude"),
	}
	result := []BackupEntry{}
	for _, dir := range dirs {
		entries, err := fsys.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			match := backupPattern.FindStringSubmatch(name)
			if len(match) != 3 {
				continue
			}
			var stamp int64
			fmt.Sscanf(match[2], "%d", &stamp)
			result = append(result, BackupEntry{
				Original:  filepath.Join(dir, match[1]),
				Backup:    filepath.Join(dir, name),
				Timestamp: stamp,
			})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp > result[j].Timestamp
	})
	return result, nil
}

func List(logger *output.Logger) error {
	home, err := repo.HomeDir()
	if err != nil {
		return err
	}
	backups, err := FindAll(home)
	if err != nil {
		return err
	}
	if len(backups) == 0 {
		logger.Info("No backup files found")
		return nil
	}
	logger.Header("Backup Files")
	for _, b := range backups {
		logger.Info(repo.ContractHome(b.Backup))
	}
	return nil
}

func Restore(path string, dryRun bool, logger *output.Logger) error {
	home, err := repo.HomeDir()
	if err != nil {
		return err
	}
	resolved, err := repo.ExpandHome(path)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Clean(resolved)
	}
	if err := repo.ValidateWithinBase(resolved, home); err != nil {
		return err
	}
	name := filepath.Base(resolved)
	match := backupPattern.FindStringSubmatch(name)
	if len(match) != 3 {
		return fmt.Errorf("invalid backup file name: %s", name)
	}
	original := filepath.Join(filepath.Dir(resolved), match[1])
	if dryRun {
		logger.DryRun(fmt.Sprintf("Would restore %s", repo.ContractHome(original)))
		return nil
	}
	_ = fsys.Remove(original)
	if err := fsys.Rename(resolved, original); err != nil {
		return err
	}
	logger.Success(fmt.Sprintf("Restored %s", repo.ContractHome(original)))
	return nil
}

func Clean(cfg config.BackupConfig, dryRun bool, logger *output.Logger) (int, error) {
	home, err := repo.HomeDir()
	if err != nil {
		return 0, err
	}
	backups, err := FindAll(home)
	if err != nil {
		return 0, err
	}
	cutoff := clk.Now().Add(-time.Duration(cfg.MaxAge) * 24 * time.Hour).UnixMilli()
	grouped := map[string][]BackupEntry{}
	for _, b := range backups {
		grouped[b.Original] = append(grouped[b.Original], b)
	}
	removed := 0
	for _, perFile := range grouped {
		sort.Slice(perFile, func(i, j int) bool { return perFile[i].Timestamp > perFile[j].Timestamp })
		for idx, item := range perFile {
			if idx < cfg.MaxCount && item.Timestamp >= cutoff {
				continue
			}
			if dryRun {
				logger.DryRun(fmt.Sprintf("Would remove %s", repo.ContractHome(item.Backup)))
				continue
			}
			if err := fsys.Remove(item.Backup); err == nil {
				removed++
			}
		}
	}
	return removed, nil
}

func Rollback(cfg config.Config, opts Options, logger *output.Logger) error {
	if err := hooks.Run("pre_rollback", cfg, hooks.Options{DryRun: opts.DryRun, SkipHooks: opts.SkipHooks}, logger); err != nil {
		return err
	}
	lastRun, err := LoadLastRun()
	if err != nil {
		return err
	}
	if lastRun == nil {
		return fmt.Errorf("no previous run state found")
	}
	for _, link := range lastRun.Symlinks {
		if opts.DryRun {
			logger.DryRun(fmt.Sprintf("Would remove symlink %s", repo.ContractHome(link.Target)))
			continue
		}
		_ = fsys.Remove(link.Target)
	}
	for _, backup := range lastRun.Backups {
		if opts.DryRun {
			logger.DryRun(fmt.Sprintf("Would restore %s", repo.ContractHome(backup.Original)))
			continue
		}
		_ = fsys.Remove(backup.Original)
		if err := fsys.Rename(backup.Backup, backup.Original); err != nil {
			logger.Warn(fmt.Sprintf("Failed to restore %s: %v", repo.ContractHome(backup.Original), err))
		}
	}
	if !opts.DryRun {
		if path, err := state.LastRunPath(); err == nil {
			_ = fsys.Remove(path)
		}
	}
	if err := hooks.Run("post_rollback", cfg, hooks.Options{DryRun: opts.DryRun, SkipHooks: opts.SkipHooks}, logger); err != nil {
		return err
	}
	logger.Success("Rollback complete")
	return nil
}
