package hooks

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/platform"
	"github.com/alexcatdad/paw/internal/repo"
)

const hookTimeout = 5 * time.Minute

type Options struct {
	DryRun    bool
	SkipHooks bool
	// Confirm, when true, prompts the user before executing each hook.
	// Used during paw init to guard against supply-chain attacks from
	// freshly cloned, untrusted repos.
	Confirm bool
	// Stdin is used for confirmation prompts. When nil, os.Stdin is used.
	Stdin io.Reader
}

// filterEnv returns a copy of env with every entry whose key starts with
// prefix removed. This prevents duplicates when the caller later appends
// its own authoritative values for that prefix.
func filterEnv(env []string, prefix string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			out = append(out, e)
		}
	}
	return out
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

	if opts.Confirm {
		fmt.Printf("Run hook '%s': %s? [y/N] ", name, cmdText)
		stdin := opts.Stdin
		if stdin == nil {
			stdin = os.Stdin
		}
		scanner := bufio.NewScanner(stdin)
		scanner.Scan()
		answer := strings.TrimSpace(scanner.Text())
		if answer != "y" && answer != "Y" {
			logger.Warn(fmt.Sprintf("hook %s skipped by user", name))
			return nil
		}
	}

	home, _ := repo.HomeDir()
	repoDir, _ := repo.RepoDir()
	base := filterEnv(os.Environ(), "PAW_")
	env := append(base,
		"PAW_PLATFORM="+platform.Current(),
		"PAW_HOME="+home,
		"PAW_REPO="+repoDir,
	)

	ctx, cancel := context.WithTimeout(context.Background(), hookTimeout)
	defer cancel()
	if err := getRunner().RunWithContext(ctx, "sh", []string{"-c", cmdText}, execx.CommandOptions{
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
