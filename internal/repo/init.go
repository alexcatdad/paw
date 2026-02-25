package repo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/alexcatdad/paw/internal/output"
)

const cloneTimeout = 3 * time.Minute

var (
	httpsRepoPattern = regexp.MustCompile(`^https?://[^/]+/.+`)
	sshRepoPattern   = regexp.MustCompile(`^git@[^:]+:.+`)
)

type InitOptions struct {
	DryRun bool
	Force  bool
	Path   string
}

func RunInit(repoURL string, opts InitOptions, logger *output.Logger) (bool, error) {
	if !isValidRepoURL(repoURL) {
		return false, fmt.Errorf("invalid repository URL format")
	}
	cfg, err := LoadPawConfig()
	if err != nil {
		return false, err
	}
	if cfg != nil && !opts.Force {
		return false, fmt.Errorf("already initialized with %s (use --force)", cfg.RepoURL)
	}
	home, err := HomeDir()
	if err != nil {
		return false, err
	}
	clonePath := opts.Path
	if strings.TrimSpace(clonePath) == "" {
		clonePath = filepath.Join(home, "dotfiles")
	}
	logger.Table(map[string]string{"Repository": repoURL, "Clone to": clonePath})
	if opts.DryRun {
		logger.DryRun("Would clone repository and save paw config")
		return true, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), cloneTimeout)
	defer cancel()

	if st, err := os.Stat(clonePath); err == nil && st.IsDir() {
		remote, remoteErr := getRunner().Output("git", "-C", clonePath, "remote", "get-url", "origin")
		if remoteErr == nil {
			existing := strings.TrimSpace(string(remote))
			if normalizeGitURL(existing) == normalizeGitURL(repoURL) {
				logger.Info("Repository already exists, pulling latest")
				_ = getRunner().RunContext(ctx, "git", "-C", clonePath, "pull", "--rebase")
			} else {
				return false, fmt.Errorf("directory exists with different remote: %s", existing)
			}
		} else {
			return false, fmt.Errorf("directory exists but is not a git repository: %s", clonePath)
		}
	} else {
		if err := getRunner().RunContext(ctx, "git", "clone", repoURL, clonePath); err != nil {
			return false, err
		}
	}

	stored := strings.Replace(clonePath, home, "~", 1)
	if err := SavePawConfig(PawConfig{DotfilesRepo: stored, RepoURL: repoURL}); err != nil {
		return false, err
	}
	logger.Success("paw initialization complete")
	return true, nil
}

func normalizeGitURL(url string) string {
	url = strings.TrimSpace(strings.TrimSuffix(url, ".git"))
	if m := regexp.MustCompile(`^git@([^:]+):(.+)$`).FindStringSubmatch(url); len(m) == 3 {
		return strings.ToLower(m[1] + "/" + m[2])
	}
	if m := regexp.MustCompile(`^https?://(.+)$`).FindStringSubmatch(url); len(m) == 2 {
		return strings.ToLower(m[1])
	}
	return strings.ToLower(url)
}

func isValidRepoURL(url string) bool {
	return httpsRepoPattern.MatchString(url) || sshRepoPattern.MatchString(url)
}
