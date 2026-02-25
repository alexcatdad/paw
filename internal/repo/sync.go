package repo

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	gitOpTimeout = 2 * time.Minute
)

type SyncResult struct {
	RepoUpdated    bool
	LinksRefreshed bool
	PawUpdated     bool
	FilesUpdated   int
}

func RepoStatus(repoDir string) (behind bool, ahead bool, commits int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitOpTimeout)
	defer cancel()
	_ = getRunner().RunContext(ctx, "git", "-C", repoDir, "fetch", "origin", "--quiet")

	out, err := getRunner().CombinedOutput("git", "-C", repoDir, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err != nil {
		return false, false, 0, nil
	}
	var a, b int
	_, _ = fmt.Sscanf(strings.TrimSpace(string(out)), "%d %d", &a, &b)
	return b > 0, a > 0, b, nil
}

func PullRepo(repoDir string, dryRun bool) ([]string, error) {
	if dryRun {
		return []string{}, nil
	}
	oldHead, _ := getRunner().Output("git", "-C", repoDir, "rev-parse", "HEAD")
	statusOut, _ := getRunner().Output("git", "-C", repoDir, "status", "--porcelain")
	hasChanges := strings.TrimSpace(string(statusOut)) != ""
	if hasChanges {
		_ = getRunner().Run("git", "-C", repoDir, "stash", "push", "-m", "paw-sync-auto-stash")
	}

	pullCtx, pullCancel := context.WithTimeout(context.Background(), gitOpTimeout)
	defer pullCancel()
	pullOut, err := getRunner().CombinedOutputContext(pullCtx, "git", "-C", repoDir, "pull", "--rebase", "--quiet")
	if err != nil {
		stderr := strings.ToLower(string(pullOut))
		if strings.Contains(stderr, "conflict") {
			_ = getRunner().Run("git", "-C", repoDir, "rebase", "--abort")
			if hasChanges {
				_ = getRunner().Run("git", "-C", repoDir, "stash", "pop")
			}
			return nil, nil
		}
		return nil, err
	}
	if hasChanges {
		_ = getRunner().Run("git", "-C", repoDir, "stash", "pop")
	}
	newHead, _ := getRunner().Output("git", "-C", repoDir, "rev-parse", "HEAD")
	if strings.TrimSpace(string(oldHead)) == strings.TrimSpace(string(newHead)) {
		return []string{}, nil
	}
	diff, err := getRunner().Output("git", "-C", repoDir, "diff", "--name-only", strings.TrimSpace(string(oldHead)), strings.TrimSpace(string(newHead)))
	if err != nil {
		return []string{}, nil
	}
	lines := []string{}
	for _, line := range strings.Split(strings.TrimSpace(string(diff)), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return lines, nil
}

func ShouldRefreshLinks(changed []string) bool {
	for _, file := range changed {
		if strings.HasPrefix(file, "home/") || strings.HasPrefix(file, "templates/") || file == "paw.toml" {
			return true
		}
	}
	return false
}
