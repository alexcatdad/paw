package repo

import (
	"context"
	"fmt"
	"strings"

	"github.com/alexcatdad/paw/internal/output"
)

type PushOptions struct {
	DryRun  bool
	Verbose bool
}

func RunPush(message string, opts PushOptions, logger *output.Logger) error {
	repoPath, err := DotfilesPath()
	if err != nil {
		return err
	}
	statusOut, err := getRunner().CombinedOutput("git", "-C", repoPath, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("git status failed: %s", strings.TrimSpace(string(statusOut)))
	}
	changes := strings.TrimSpace(string(statusOut))
	if changes == "" {
		logger.Info("No changes to push")
		return nil
	}
	if opts.Verbose {
		logger.Info(changes)
	}
	if strings.TrimSpace(message) == "" {
		message = fmt.Sprintf("Update dotfiles (%s)", getClk().Now().Format("2006-01-02"))
	}
	if opts.DryRun {
		logger.DryRun("Would stage, commit, and push changes")
		return nil
	}
	if out, err := getRunner().CombinedOutput("git", "-C", repoPath, "add", "-A"); err != nil {
		return fmt.Errorf("git add failed: %s", strings.TrimSpace(string(out)))
	}
	if out, err := getRunner().CombinedOutput("git", "-C", repoPath, "commit", "-m", message); err != nil {
		if strings.Contains(strings.ToLower(string(out)), "nothing to commit") {
			return nil
		}
		return fmt.Errorf("git commit failed: %s", strings.TrimSpace(string(out)))
	}

	pushCtx, pushCancel := context.WithTimeout(context.Background(), gitOpTimeout)
	defer pushCancel()
	if out, err := getRunner().CombinedOutputContext(pushCtx, "git", "-C", repoPath, "push"); err != nil {
		return fmt.Errorf("git push failed: %s", strings.TrimSpace(string(out)))
	}
	logger.Success("Pushed changes")
	return nil
}
