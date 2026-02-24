package packages

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/platform"
)

var packageNamePattern = regexp.MustCompile(`^[@a-zA-Z0-9_\/-]{1,255}$`)

type Options struct {
	DryRun bool
}

type Result struct {
	Installed []string
	Failed    []string
	Skipped   []string
}

func InstallAll(cfg config.PackageConfig, opts Options, logger *output.Logger) Result {
	result := Result{Installed: []string{}, Failed: []string{}, Skipped: []string{}}
	current := platform.Current()

	if current == platform.Linux || current == platform.WSL {
		aptPkgs := cfg.LinuxAPT
		if current == platform.WSL && len(cfg.WSLAPT) > 0 {
			aptPkgs = cfg.WSLAPT
		}
		if len(aptPkgs) > 0 {
			logger.Subheader("Installing apt packages")
			for _, pkg := range aptPkgs {
				if installApt(pkg, opts.DryRun, logger) {
					result.Installed = append(result.Installed, pkg)
				} else {
					result.Failed = append(result.Failed, pkg)
				}
			}
		}
	}

	brewPath, err := ensureBrew(opts.DryRun, logger)
	if err != nil {
		logger.Warn(fmt.Sprintf("Homebrew unavailable: %v", err))
	}

	pkgs := append([]string{}, cfg.Common...)
	switch current {
	case platform.Darwin:
		pkgs = append(pkgs, cfg.Darwin...)
	case platform.Linux:
		pkgs = append(pkgs, cfg.LinuxBrew...)
	case platform.WSL:
		if len(cfg.WSLBrew) > 0 {
			pkgs = append(pkgs, cfg.WSLBrew...)
		} else {
			pkgs = append(pkgs, cfg.LinuxBrew...)
		}
	}

	if brewPath != "" {
		if !opts.DryRun {
			_ = runCmd(brewPath, "update")
		}
		logger.Subheader("Installing brew packages")
		for _, pkg := range pkgs {
			ok := installBrew(pkg, brewPath, opts.DryRun, logger)
			if ok {
				result.Installed = append(result.Installed, pkg)
			} else {
				result.Failed = append(result.Failed, pkg)
			}
		}
	}

	if current == platform.Linux || current == platform.WSL {
		_ = installLinuxFont(opts.DryRun, logger)
	}

	return result
}

func Check(cfg config.PackageConfig) (installed []string, missing []string) {
	pkgs := append([]string{}, cfg.Common...)
	switch platform.Current() {
	case platform.Darwin:
		pkgs = append(pkgs, cfg.Darwin...)
	case platform.Linux:
		pkgs = append(pkgs, cfg.LinuxBrew...)
	case platform.WSL:
		if len(cfg.WSLBrew) > 0 {
			pkgs = append(pkgs, cfg.WSLBrew...)
		} else {
			pkgs = append(pkgs, cfg.LinuxBrew...)
		}
	}
	for _, pkg := range pkgs {
		if platform.CommandExists(pkg) {
			installed = append(installed, pkg)
		} else {
			missing = append(missing, pkg)
		}
	}
	return installed, missing
}

func ensureBrew(dryRun bool, logger *output.Logger) (string, error) {
	if path, err := runner.LookPath("brew"); err == nil {
		return path, nil
	}
	if dryRun {
		logger.DryRun("Would install Homebrew")
		if platform.Current() == platform.Darwin {
			return "/opt/homebrew/bin/brew", nil
		}
		return "/home/linuxbrew/.linuxbrew/bin/brew", nil
	}
	logger.Info("Homebrew not found. Installing...")
	if err := runner.RunWith("/bin/bash", []string{"-c", "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"}, execx.CommandOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		return "", err
	}
	if path, err := runner.LookPath("brew"); err == nil {
		return path, nil
	}
	return "", errors.New("homebrew install finished but brew not found")
}

func installBrew(pkg string, brewPath string, dryRun bool, logger *output.Logger) bool {
	if !packageNamePattern.MatchString(pkg) {
		logger.Error(fmt.Sprintf("Invalid package name: %s", pkg))
		return false
	}
	if runCmd(brewPath, "list", "--formula", pkg) == nil || runCmd(brewPath, "list", "--cask", pkg) == nil {
		logger.Info(fmt.Sprintf("Skipping %s (already installed)", pkg))
		return true
	}
	if dryRun {
		logger.DryRun(fmt.Sprintf("Would install %s", pkg))
		return true
	}
	if err := runCmd(brewPath, "install", pkg); err == nil {
		logger.Success(fmt.Sprintf("Installed %s", pkg))
		return true
	}
	if err := runCmd(brewPath, "install", "--cask", pkg); err == nil {
		logger.Success(fmt.Sprintf("Installed %s (cask)", pkg))
		return true
	}
	logger.Error(fmt.Sprintf("Failed to install %s", pkg))
	return false
}

func installApt(pkg string, dryRun bool, logger *output.Logger) bool {
	if !packageNamePattern.MatchString(pkg) {
		logger.Error(fmt.Sprintf("Invalid package name: %s", pkg))
		return false
	}
	if runCmd("dpkg", "-s", pkg) == nil {
		logger.Info(fmt.Sprintf("Skipping %s (already installed)", pkg))
		return true
	}
	if dryRun {
		logger.DryRun(fmt.Sprintf("Would install via apt: %s", pkg))
		return true
	}
	if err := runCmd("sudo", "apt", "install", "-y", pkg); err != nil {
		logger.Error(fmt.Sprintf("Failed apt install for %s: %v", pkg, err))
		return false
	}
	logger.Success(fmt.Sprintf("Installed %s", pkg))
	return true
}

func installLinuxFont(dryRun bool, logger *output.Logger) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	fontDir := filepathJoin(home, ".local", "share", "fonts")
	if output, err := runner.CombinedOutput("fc-list"); err == nil {
		if strings.Contains(strings.ToLower(string(output)), "firacode nerd") {
			logger.Info("FiraCode Nerd Font already installed")
			return nil
		}
	}
	if dryRun {
		logger.DryRun("Would install FiraCode Nerd Font")
		return nil
	}
	tmp := filepathJoin("/tmp", fmt.Sprintf("paw-font-%d", os.Getpid()))
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	zipPath := filepathJoin(tmp, "FiraCode.zip")
	if err := runCmd("curl", "-fsSL", "https://github.com/ryanoasis/nerd-fonts/releases/latest/download/FiraCode.zip", "-o", zipPath); err != nil {
		return err
	}
	if err := runCmd("unzip", "-q", zipPath, "-d", tmp); err != nil {
		return err
	}
	if err := os.MkdirAll(fontDir, 0o755); err != nil {
		return err
	}
	_ = runCmd("sh", "-c", fmt.Sprintf("cp %s/*.ttf %s/ 2>/dev/null || true", tmp, fontDir))
	_ = runCmd("fc-cache", "-f", fontDir)
	logger.Success("Installed FiraCode Nerd Font")
	return nil
}

func runCmd(name string, args ...string) error {
	return runner.Run(name, args...)
}

func filepathJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	current := parts[0]
	for _, p := range parts[1:] {
		if strings.HasSuffix(current, "/") {
			current += p
		} else {
			current += "/" + p
		}
	}
	return current
}
