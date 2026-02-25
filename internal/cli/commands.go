package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexcatdad/paw/internal/app"
	"github.com/alexcatdad/paw/internal/audit"
	"github.com/alexcatdad/paw/internal/backup"
	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/drift"
	"github.com/alexcatdad/paw/internal/hooks"
	"github.com/alexcatdad/paw/internal/packages"
	"github.com/alexcatdad/paw/internal/platform"
	"github.com/alexcatdad/paw/internal/repo"
	"github.com/alexcatdad/paw/internal/scaffold"
	"github.com/alexcatdad/paw/internal/symlink"
	"github.com/alexcatdad/paw/internal/update"
	"github.com/spf13/cobra"
)

func newInstallCommand(opts *app.GlobalOptions) *cobra.Command {
	return &cobra.Command{Use: "install", Short: "Install packages and create symlinks", RunE: func(cmd *cobra.Command, args []string) error {
		return runInstall(cmd, opts)
	}}
}

func newLinkCommand(opts *app.GlobalOptions) *cobra.Command {
	return &cobra.Command{Use: "link", Short: "Create symlinks only", RunE: func(cmd *cobra.Command, args []string) error {
		return runLink(cmd, opts)
	}}
}

func newUnlinkCommand(opts *app.GlobalOptions) *cobra.Command {
	return &cobra.Command{Use: "unlink", Short: "Remove managed symlinks", RunE: func(cmd *cobra.Command, args []string) error {
		return runUnlink(cmd, opts)
	}}
}

func newStatusCommand(opts *app.GlobalOptions) *cobra.Command {
	return &cobra.Command{Use: "status", Short: "Show current symlink and package state", RunE: func(cmd *cobra.Command, args []string) error {
		return runStatus(cmd, opts)
	}}
}

func newDriftCommand(opts *app.GlobalOptions) *cobra.Command {
	var scope string
	driftCmd := &cobra.Command{
		Use:   "drift",
		Short: "Inspect and apply drift from system state to repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDriftStatus(cmd, opts, scope)
		},
	}
	driftCmd.PersistentFlags().StringVar(&scope, "scope", string(drift.ScopeAll), "Drift scope: all|files|packages")

	driftCmd.AddCommand(
		&cobra.Command{
			Use:   "status",
			Short: "Inspect drift without applying changes",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runDriftStatus(cmd, opts, scope)
			},
		},
		&cobra.Command{
			Use:   "apply",
			Short: "Apply drift into repo and relink managed targets",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runDriftApply(cmd, opts, scope)
			},
		},
	)
	return driftCmd
}

func newBackupCommand(opts *app.GlobalOptions) *cobra.Command {
	backupCmd := &cobra.Command{Use: "backup", Short: "Backup management"}
	backupCmd.AddCommand(&cobra.Command{Use: "list", Short: "List backups", RunE: func(cmd *cobra.Command, args []string) error {
		logger := newLogger(opts)
		return backup.List(logger)
	}})
	backupCmd.AddCommand(&cobra.Command{Use: "restore <backup-file>", Short: "Restore specific backup", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		logger := newLogger(opts)
		return backup.Restore(args[0], opts.DryRun, logger)
	}})
	backupCmd.AddCommand(&cobra.Command{Use: "clean", Short: "Clean old backups", RunE: func(cmd *cobra.Command, args []string) error {
		logger := newLogger(opts)
		_, _, cfg, _, err := loadConfigEntries(logger)
		if err != nil {
			return err
		}
		removed, err := backup.Clean(cfg.Backup, opts.DryRun, logger)
		if err != nil {
			return err
		}
		logger.Info(fmt.Sprintf("Removed %d backup(s)", removed))
		return nil
	}})
	return backupCmd
}

func newRollbackCommand(opts *app.GlobalOptions) *cobra.Command {
	return &cobra.Command{Use: "rollback", Short: "Restore backups and remove links from last run", RunE: func(cmd *cobra.Command, args []string) error {
		logger := newLogger(opts)
		_, _, cfg, _, err := loadConfigEntries(logger)
		if err != nil {
			return err
		}
		return backup.Rollback(cfg, backup.Options{DryRun: opts.DryRun, SkipHooks: opts.SkipHooks}, logger)
	}}
}

func newSyncCommand(opts *app.GlobalOptions) *cobra.Command {
	return &cobra.Command{Use: "sync", Short: "Pull repo and refresh links", RunE: func(cmd *cobra.Command, args []string) error {
		logger := newLogger(opts)
		repoDir, _, cfg, entries, err := loadConfigEntries(logger)
		if err != nil {
			return err
		}
		if err := hooks.Run("pre_sync", cfg, hooks.Options{DryRun: opts.DryRun, SkipHooks: opts.SkipHooks}, logger); err != nil {
			return err
		}
		if !opts.SkipUpdate {
			latest, _ := update.CheckForUpdate(Version(), false)
			if latest != "" {
				logger.Info(fmt.Sprintf("paw update available: v%s", latest))
				if opts.AutoUpdate {
					if err := update.Perform(Version(), update.Options{DryRun: opts.DryRun, SkipVerify: opts.SkipVerify}, logger); err != nil {
						logger.Warn(err.Error())
					}
				}
			}
		}
		behind, _, commits, _ := repo.RepoStatus(repoDir)
		if !behind {
			logger.Success("Dotfiles repo is up to date")
			return nil
		}
		logger.Info(fmt.Sprintf("%d commit(s) behind origin", commits))
		changed, err := repo.PullRepo(repoDir, opts.DryRun)
		if err != nil {
			return err
		}
		if len(changed) > 0 && repo.ShouldRefreshLinks(changed) {
			linkOpts := symlink.LinkOptions{DryRun: opts.DryRun, Force: true, NoInteractive: true}
			states, err := symlink.Create(entries, linkOpts, logger)
			if err != nil {
				return err
			}
			_ = saveLastRun("sync", states)
		}
		if err := hooks.Run("post_sync", cfg, hooks.Options{DryRun: opts.DryRun, SkipHooks: opts.SkipHooks}, logger); err != nil {
			return err
		}
		logger.Success("Sync complete")
		return nil
	}}
}

func newInitCommand(opts *app.GlobalOptions) *cobra.Command {
	return &cobra.Command{Use: "init <repo-url>", Short: "Clone dotfiles repo and configure paw", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		logger := newLogger(opts)
		repoURL := args[0]
		ok, err := repo.RunInit(repoURL, repo.InitOptions{DryRun: opts.DryRun, Force: opts.Force, Path: opts.Path}, logger)
		if err != nil {
			return err
		}
		if ok && !opts.DryRun {
			clonePath := opts.Path
			if strings.TrimSpace(clonePath) == "" {
				home, _ := repo.HomeDir()
				clonePath = filepath.Join(home, "dotfiles")
			}
			_ = os.Setenv("PAW_REPO", clonePath)
			return runInstall(cmd, opts)
		}
		return nil
	}}
}

func newPushCommand(opts *app.GlobalOptions) *cobra.Command {
	return &cobra.Command{Use: "push [message]", Short: "Commit and push dotfiles changes", RunE: func(cmd *cobra.Command, args []string) error {
		logger := newLogger(opts)
		repoDir, _, cfg, _, err := loadConfigEntries(logger)
		if err != nil {
			return err
		}
		_ = repoDir
		if err := hooks.Run("pre_push", cfg, hooks.Options{DryRun: opts.DryRun, SkipHooks: opts.SkipHooks}, logger); err != nil {
			return err
		}
		message := strings.Join(args, " ")
		if err := repo.RunPush(message, repo.PushOptions{DryRun: opts.DryRun, Verbose: opts.Verbose}, logger); err != nil {
			return err
		}
		if err := hooks.Run("post_push", cfg, hooks.Options{DryRun: opts.DryRun, SkipHooks: opts.SkipHooks}, logger); err != nil {
			return err
		}
		return nil
	}}
}

func newUpdateCommand(opts *app.GlobalOptions) *cobra.Command {
	return &cobra.Command{Use: "update", Short: "Self-update paw binary", RunE: func(cmd *cobra.Command, args []string) error {
		logger := newLogger(opts)
		return update.Perform(Version(), update.Options{DryRun: opts.DryRun, SkipVerify: opts.SkipVerify, ForceCheck: true}, logger)
	}}
}

func newDoctorCommand(opts *app.GlobalOptions) *cobra.Command {
	return &cobra.Command{Use: "doctor", Short: "Health check and diagnostics", RunE: func(cmd *cobra.Command, args []string) error {
		logger := newLogger(opts)
		repoDir, homeDir, cfg, entries, err := loadConfigEntries(logger)
		if err != nil {
			return err
		}
		logger.Header("Dotfiles Doctor")
		printSystemTable(logger, repoDir, homeDir)
		states, err := symlink.Status(entries)
		if err != nil {
			return err
		}
		conflicts := 0
		missing := 0
		for _, st := range states {
			if st.Status == symlink.StatusConflict {
				conflicts++
			}
			if st.Status == symlink.StatusMissing || st.Status == symlink.StatusSourceMissing {
				missing++
			}
		}
		logger.Info(fmt.Sprintf("Symlink conflicts: %d", conflicts))
		logger.Info(fmt.Sprintf("Symlink missing: %d", missing))
		required := []string{"git", "zsh", "curl", "nano", "ssh", "tar", "gzip"}
		issues := 0
		for _, tool := range required {
			if platform.CommandExists(tool) {
				logger.Success(tool)
			} else {
				logger.Error(tool + " - NOT FOUND")
				issues++
			}
		}
		installed, missingPkgs := packages.Check(cfg.Packages)
		logger.Info(fmt.Sprintf("Optional tools installed: %d, missing: %d", len(installed), len(missingPkgs)))
		if opts.Verbose && len(missingPkgs) > 0 {
			logger.Warn("Missing optional tools: " + strings.Join(missingPkgs, ", "))
		}
		if issues == 0 {
			logger.Success("All checks passed")
		} else {
			logger.Warn(fmt.Sprintf("Found %d issue(s)", issues))
		}
		return nil
	}}
}

func newAuditCommand(opts *app.GlobalOptions) *cobra.Command {
	return &cobra.Command{Use: "audit", Short: "Audit repository structure and completeness", RunE: func(cmd *cobra.Command, args []string) error {
		logger := newLogger(opts)
		repoDir, _, cfg, _, err := loadConfigEntries(logger)
		if err != nil {
			return err
		}
		result, err := audit.Run(repoDir, &cfg)
		if err != nil {
			return err
		}
		audit.Print(result, audit.Options{JSON: opts.JSON, Verbose: opts.Verbose}, logger)
		return nil
	}}
}

func newScaffoldCommand(opts *app.GlobalOptions) *cobra.Command {
	return &cobra.Command{Use: "scaffold [name...]", Short: "Generate missing config templates", RunE: func(cmd *cobra.Command, args []string) error {
		logger := newLogger(opts)
		repoDir, err := repo.RepoDir()
		if err != nil {
			return err
		}
		if len(args) == 0 || (len(args) == 1 && strings.EqualFold(args[0], "list")) {
			scaffold.List(logger)
			return nil
		}
		created, err := scaffold.Create(repoDir, args, opts.DryRun, opts.Force, logger)
		if err != nil {
			return err
		}
		if opts.DryRun {
			logger.Info(fmt.Sprintf("Would create %d config(s)", created))
		} else {
			logger.Success(fmt.Sprintf("Created %d config(s)", created))
		}
		return nil
	}}
}

func newMigrateCommand(opts *app.GlobalOptions) *cobra.Command {
	var source string
	var outputPath string
	cmd := &cobra.Command{Use: "migrate-ts-config", Short: "Migrate dotfiles.config.ts to paw.toml", RunE: func(cmd *cobra.Command, args []string) error {
		logger := newLogger(opts)
		repoDir, err := repo.RepoDir()
		if err != nil {
			return err
		}
		if strings.TrimSpace(source) == "" {
			source = filepath.Join(repoDir, "dotfiles.config.ts")
		}
		if strings.TrimSpace(outputPath) == "" {
			outputPath = filepath.Join(repoDir, "paw.toml")
		}
		if opts.DryRun {
			logger.DryRun(fmt.Sprintf("Would migrate %s -> %s", source, outputPath))
			return nil
		}
		result, err := config.MigrateTSConfig(source, outputPath)
		if err != nil {
			return err
		}
		logger.Success(fmt.Sprintf("Created %s", outputPath))
		logger.Info(fmt.Sprintf("Migrated symlinks: %d, templates: %d", len(result.Symlinks), len(result.Templates)))
		return nil
	}}
	cmd.Flags().StringVar(&source, "source", "", "Path to dotfiles.config.ts")
	cmd.Flags().StringVar(&outputPath, "output", "", "Path for paw.toml")
	return cmd
}

func runInstall(cmd *cobra.Command, opts *app.GlobalOptions) error {
	logger := newLogger(opts)
	repoDir, homeDir, cfg, entries, err := loadConfigEntries(logger)
	if err != nil {
		return err
	}
	logger.Header("Dotfiles Install")
	printSystemTable(logger, repoDir, homeDir)
	if err := hooks.Run("pre_install", cfg, hooks.Options{DryRun: opts.DryRun, SkipHooks: opts.SkipHooks}, logger); err != nil {
		return err
	}
	if !opts.SkipPackages {
		result := packages.InstallAll(cfg.Packages, packages.Options{DryRun: opts.DryRun}, logger)
		logger.Info(fmt.Sprintf("Installed: %d, Failed: %d", len(result.Installed), len(result.Failed)))
	}
	states, err := symlink.Create(entries, symlink.LinkOptions{DryRun: opts.DryRun, Force: opts.Force, NoInteractive: opts.NoInteractive}, logger)
	if err != nil {
		return err
	}
	if err := generateTemplates(repoDir, cfg, opts.DryRun, logger); err != nil {
		return err
	}
	if err := hooks.Run("post_install", cfg, hooks.Options{DryRun: opts.DryRun, SkipHooks: opts.SkipHooks}, logger); err != nil {
		return err
	}
	if !opts.DryRun {
		if err := saveLastRun("install", states); err != nil {
			logger.Warn("Failed to save last run state: " + err.Error())
		}
	}
	linked := 0
	conflicts := 0
	missing := 0
	for _, st := range states {
		if st.Status == symlink.StatusLinked || st.Status == symlink.StatusBackup {
			linked++
		}
		if st.Status == symlink.StatusConflict {
			conflicts++
		}
		if st.Status == symlink.StatusSourceMissing {
			missing++
		}
	}
	logger.Table(map[string]string{
		"Symlinks created": fmt.Sprintf("%d", linked),
		"Conflicts":        fmt.Sprintf("%d", conflicts),
		"Missing sources":  fmt.Sprintf("%d", missing),
	})
	logger.Success("Installation complete")
	return nil
}

func runLink(cmd *cobra.Command, opts *app.GlobalOptions) error {
	logger := newLogger(opts)
	repoDir, _, cfg, entries, err := loadConfigEntries(logger)
	if err != nil {
		return err
	}
	_ = repoDir
	if err := hooks.Run("pre_link", cfg, hooks.Options{DryRun: opts.DryRun, SkipHooks: opts.SkipHooks}, logger); err != nil {
		return err
	}
	states, err := symlink.Create(entries, symlink.LinkOptions{DryRun: opts.DryRun, Force: opts.Force, NoInteractive: opts.NoInteractive}, logger)
	if err != nil {
		return err
	}
	if err := generateTemplates(repoDir, cfg, opts.DryRun, logger); err != nil {
		return err
	}
	if err := hooks.Run("post_link", cfg, hooks.Options{DryRun: opts.DryRun, SkipHooks: opts.SkipHooks}, logger); err != nil {
		return err
	}
	if !opts.DryRun {
		_ = saveLastRun("link", states)
	}
	logger.Success("Link complete")
	return nil
}

func runUnlink(cmd *cobra.Command, opts *app.GlobalOptions) error {
	logger := newLogger(opts)
	_, _, _, entries, err := loadConfigEntries(logger)
	if err != nil {
		return err
	}
	if err := symlink.Remove(entries, symlink.LinkOptions{DryRun: opts.DryRun}, logger); err != nil {
		return err
	}
	logger.Success("Unlink complete")
	return nil
}

func runStatus(cmd *cobra.Command, opts *app.GlobalOptions) error {
	logger := newLogger(opts)
	repoDir, homeDir, cfg, entries, err := loadConfigEntries(logger)
	if err != nil {
		return err
	}
	logger.Header("Dotfiles Status")
	printSystemTable(logger, repoDir, homeDir)
	states, err := symlink.Status(entries)
	if err != nil {
		return err
	}
	printStatusStates(states, logger)
	installed, missing := packages.Check(cfg.Packages)
	logger.Info(fmt.Sprintf("Installed packages: %d, missing: %d", len(installed), len(missing)))
	if opts.Verbose && len(missing) > 0 {
		logger.Warn("Missing packages: " + strings.Join(missing, ", "))
	}
	last, _ := backup.LoadLastRun()
	if last != nil {
		logger.Table(map[string]string{
			"Last command": last.Command,
			"Last time":    last.Timestamp,
			"Symlinks":     fmt.Sprintf("%d", len(last.Symlinks)),
			"Backups":      fmt.Sprintf("%d", len(last.Backups)),
		})
	}
	return nil
}

func runDriftStatus(cmd *cobra.Command, opts *app.GlobalOptions, scopeValue string) error {
	logger := newLogger(opts)
	repoDir, homeDir, _, entries, err := loadConfigEntries(logger)
	if err != nil {
		return err
	}
	scope, err := drift.ParseScope(scopeValue)
	if err != nil {
		return err
	}
	report, err := drift.Inspect(drift.Context{
		Scope:    scope,
		RepoPath: repoDir,
		HomePath: homeDir,
		Entries:  entries,
		Logger:   logger,
	})
	if err != nil {
		return err
	}
	if opts.JSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	logger.Header("Dotfiles Drift Status")
	printSystemTable(logger, repoDir, homeDir)
	logger.Table(map[string]string{
		"Scope":         string(scope),
		"Total drift":   fmt.Sprintf("%d", report.Summary.DriftCount),
		"Files drift":   fmt.Sprintf("%d", report.Summary.FilesDrift),
		"Package drift": fmt.Sprintf("%d", report.Summary.PackageDrift),
	})
	if len(report.Findings) == 0 {
		logger.Success("No findings")
		return nil
	}
	for _, finding := range report.Findings {
		target := finding.TargetPath
		if target == "" {
			target = finding.SourcePath
		}
		applyability := "no"
		if finding.CanApply {
			applyability = "yes"
		}
		logger.Info(fmt.Sprintf("%s [%s] %s (apply=%s)", finding.Kind, finding.Scope, repo.ContractHome(target), applyability))
		if finding.Details != "" {
			logger.Info("  " + finding.Details)
		}
	}
	if report.Summary.DriftCount > 0 {
		return app.WithCode(app.ExitConflict, fmt.Errorf("drift detected: %d finding(s)", report.Summary.DriftCount))
	}
	logger.Success("No drift detected")
	return nil
}

func runDriftApply(cmd *cobra.Command, opts *app.GlobalOptions, scopeValue string) error {
	logger := newLogger(opts)
	repoDir, homeDir, _, entries, err := loadConfigEntries(logger)
	if err != nil {
		return err
	}
	scope, err := drift.ParseScope(scopeValue)
	if err != nil {
		return err
	}
	result, err := drift.Apply(drift.Context{
		Scope:    scope,
		RepoPath: repoDir,
		HomePath: homeDir,
		Entries:  entries,
		DryRun:   opts.DryRun,
		Logger:   logger,
	})
	if err != nil {
		return err
	}
	if opts.JSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			return err
		}
	} else {
		logger.Header("Dotfiles Drift Apply")
		logger.Table(map[string]string{
			"Scope":           string(scope),
			"Applied":         fmt.Sprintf("%d", len(result.Applied)),
			"Skipped":         fmt.Sprintf("%d", len(result.Skipped)),
			"Failed":          fmt.Sprintf("%d", len(result.Failed)),
			"Remaining drift": fmt.Sprintf("%d", len(result.RemainingDrift)),
		})
		for _, finding := range result.Applied {
			p := finding.TargetPath
			if p == "" {
				p = finding.SourcePath
			}
			if opts.DryRun {
				logger.DryRun(fmt.Sprintf("Would apply %s on %s", finding.Kind, repo.ContractHome(p)))
			} else {
				logger.Success(fmt.Sprintf("Applied %s on %s", finding.Kind, repo.ContractHome(p)))
			}
		}
		for _, finding := range result.Skipped {
			p := finding.TargetPath
			if p == "" {
				p = finding.SourcePath
			}
			logger.Info(fmt.Sprintf("Skipped %s on %s", finding.Kind, repo.ContractHome(p)))
		}
		for _, finding := range result.Failed {
			p := finding.TargetPath
			if p == "" {
				p = finding.SourcePath
			}
			logger.Warn(fmt.Sprintf("Failed %s on %s: %s", finding.Kind, repo.ContractHome(p), finding.Details))
		}
		for _, backupEntry := range result.Backups {
			logger.Info(fmt.Sprintf("Backup created: %s", repo.ContractHome(backupEntry.Backup)))
		}
	}
	if !opts.DryRun {
		if err := saveDriftLastRun(result); err != nil {
			logger.Warn("Failed to save drift apply state: " + err.Error())
		}
	}
	if len(result.RemainingDrift) > 0 {
		return app.WithCode(app.ExitConflict, fmt.Errorf("remaining drift: %d finding(s)", len(result.RemainingDrift)))
	}
	if !opts.JSON {
		logger.Success("Drift apply complete")
	}
	return nil
}
