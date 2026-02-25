package update

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/state"
)

const repoName = "alexcatdad/paw"
const checkInterval = 24 * time.Hour

const (
	ghReleaseCheckTimeout    = 30 * time.Second
	ghReleaseDownloadTimeout = 3 * time.Minute
)

type updateState struct {
	LastCheck      string `json:"lastCheck"`
	LatestVersion  string `json:"latestVersion"`
	CurrentVersion string `json:"currentVersion"`
}

type Options struct {
	DryRun     bool
	SkipVerify bool
	ForceCheck bool
}

func CheckForUpdate(currentVersion string, force bool) (string, error) {
	if !force {
		s, _ := loadState()
		if s != nil {
			last, err := time.Parse(time.RFC3339, s.LastCheck)
			if err == nil && time.Since(last) < checkInterval {
				if compareVersions(currentVersion, s.LatestVersion) < 0 {
					return s.LatestVersion, nil
				}
				return "", nil
			}
		}
	}
	if _, err := getRunner().LookPath("gh"); err != nil {
		return "", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), ghReleaseCheckTimeout)
	defer cancel()
	out, err := getRunner().OutputContext(ctx, "gh", "release", "view", "latest", "--repo", repoName, "--json", "tagName")
	if err != nil {
		return "", nil
	}
	var payload struct {
		TagName string `json:"tagName"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", err
	}
	latest := strings.TrimPrefix(strings.TrimSpace(payload.TagName), "v")
	_ = saveState(updateState{LastCheck: getClk().Now().UTC().Format(time.RFC3339), LatestVersion: latest, CurrentVersion: currentVersion})
	if compareVersions(currentVersion, latest) < 0 {
		return latest, nil
	}
	return "", nil
}

func Perform(currentVersion string, opts Options, logger *output.Logger) error {
	if _, err := getRunner().LookPath("gh"); err != nil {
		return fmt.Errorf("gh CLI not found")
	}
	latest, err := CheckForUpdate(currentVersion, opts.ForceCheck)
	if err != nil {
		return err
	}
	if latest == "" {
		logger.Success("Already up to date")
		return nil
	}
	binaryPath, err := getRunner().LookPath("paw")
	if err != nil {
		return fmt.Errorf("paw binary not found in PATH")
	}
	asset := fmt.Sprintf("paw-%s-%s", runtime.GOOS, mapArch(runtime.GOARCH))

	// Fix M6: use os.MkdirTemp for an unpredictable temp directory
	tmpDir, err := os.MkdirTemp("", "paw-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	if opts.DryRun {
		logger.DryRun(fmt.Sprintf("Would download %s and replace %s", asset, binaryPath))
		return nil
	}
	args := []string{"release", "download", "latest", "--repo", repoName, "--pattern", asset, "--dir", tmpDir}

	// Fix I2: warn explicitly when attestation verification is unavailable
	if opts.SkipVerify {
		fmt.Fprintf(os.Stderr, "WARNING: Binary verification skipped (--skip-verify flag).\n")
	} else {
		helpOut, _ := getRunner().Output("gh", "release", "download", "--help")
		if strings.Contains(string(helpOut), "verify-attestation") {
			args = append(args, "--verify-attestation")
		} else {
			fmt.Fprintf(os.Stderr, "WARNING: gh CLI does not support attestation verification. Update gh to enable binary verification. Proceeding without verification.\n")
		}
	}

	dlCtx, dlCancel := context.WithTimeout(context.Background(), ghReleaseDownloadTimeout)
	defer dlCancel()
	if out, err := getRunner().CombinedOutputContext(dlCtx, "gh", args...); err != nil {
		return fmt.Errorf("download failed: %s", strings.TrimSpace(string(out)))
	}

	downloaded := filepath.Join(tmpDir, asset)
	backup := binaryPath + ".backup"
	if err := copyFile(binaryPath, backup); err != nil {
		return err
	}
	if err := getFsys().Rename(downloaded, binaryPath); err != nil {
		_ = getFsys().Rename(backup, binaryPath)
		return err
	}
	if err := getFsys().Chmod(binaryPath, 0o755); err != nil {
		_ = getFsys().Rename(backup, binaryPath)
		return err
	}
	if out, err := getRunner().CombinedOutput(binaryPath, "--version"); err != nil {
		_ = getFsys().Rename(backup, binaryPath)
		return fmt.Errorf("verification failed: %s", strings.TrimSpace(string(out)))
	}
	_ = getFsys().Remove(backup)
	logger.Success(fmt.Sprintf("Updated paw: v%s -> v%s", currentVersion, latest))
	return nil
}

func compareVersions(a string, b string) int {
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	max := len(as)
	if len(bs) > max {
		max = len(bs)
	}
	for i := 0; i < max; i++ {
		var ai, bi int
		if i < len(as) {
			fmt.Sscanf(as[i], "%d", &ai)
		}
		if i < len(bs) {
			fmt.Sscanf(bs[i], "%d", &bi)
		}
		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
	}
	return 0
}

func mapArch(arch string) string {
	switch arch {
	case "amd64":
		return "x64"
	case "arm64":
		return "arm64"
	default:
		return arch
	}
}

func copyFile(src string, dst string) error {
	in, err := getFsys().ReadFile(src)
	if err != nil {
		return err
	}
	return getFsys().WriteFile(dst, in, 0o755)
}

func loadState() (*updateState, error) {
	path, err := state.UpdateStatePath()
	if err != nil {
		return nil, err
	}
	data, err := getFsys().ReadFile(path)
	if err != nil {
		return nil, nil
	}
	var st updateState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func saveState(st updateState) error {
	path, err := state.UpdateStatePath()
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	// Fix I4: use 0o600 to restrict state file to owner-only
	return getFsys().WriteFile(path, payload, 0o600)
}
