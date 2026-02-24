package hooks

import (
	"fmt"
	"os"
	"strings"

	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/platform"
	"github.com/alexcatdad/paw/internal/repo"
)

type Options struct {
	DryRun    bool
	SkipHooks bool
}

func Run(name string, cfg config.Config, opts Options, logger *output.Logger) error {
	if opts.SkipHooks {
		logger.Debug("hooks skipped by --skip-hooks")
		return nil
	}
	cmdText := selectHook(name, cfg.Hooks)
	cmdText = strings.TrimSpace(cmdText)
	if cmdText == "" {
		return nil
	}
	logger.Subheader(fmt.Sprintf("Running %s hook", name))
	if opts.DryRun {
		logger.DryRun(fmt.Sprintf("Would run hook: %s", cmdText))
		return nil
	}
	home, _ := repo.HomeDir()
	repoDir, _ := repo.RepoDir()
	env := append(os.Environ(),
		"PAW_PLATFORM="+platform.Current(),
		"PAW_HOME="+home,
		"PAW_REPO="+repoDir,
	)
	if err := runner.RunWith("sh", []string{"-c", cmdText}, execx.CommandOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Env:    env,
	}); err != nil {
		return fmt.Errorf("hook %s failed: %w", name, err)
	}
	return nil
}

func selectHook(name string, hooks config.HookConfig) string {
	switch name {
	case "pre_install":
		return hooks.PreInstall
	case "post_install":
		return hooks.PostInstall
	case "pre_link":
		return hooks.PreLink
	case "post_link":
		return hooks.PostLink
	case "pre_sync":
		return hooks.PreSync
	case "post_sync":
		return hooks.PostSync
	case "pre_push":
		return hooks.PrePush
	case "post_push":
		return hooks.PostPush
	case "pre_update":
		return hooks.PreUpdate
	case "post_update":
		return hooks.PostUpdate
	case "pre_rollback":
		return hooks.PreRollback
	case "post_rollback":
		return hooks.PostRollback
	default:
		return ""
	}
}
