package packages

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/platform"
)

const (
	pkgInstallTimeout = 5 * time.Minute
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

	brewPath, err := ensureBrew(opts.DryRun, os.Stdin, logger)
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
			updateCtx, updateCancel := context.WithTimeout(context.Background(), pkgInstallTimeout)
			defer updateCancel()
			_ = runCmdContext(updateCtx, brewPath, "update")
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
		_ = installFont(cfg, brewPath, opts.DryRun, logger)
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

// ensureBrew returns the path to brew, installing it after user confirmation
// if it is not already present. stdin is used to read the confirmation answer
// so it can be injected in tests.
func ensureBrew(dryRun bool, stdin io.Reader, logger *output.Logger) (string, error) {
	if path, err := getRunner().LookPath("brew"); err == nil {
		return path, nil
	}
	if dryRun {
		logger.DryRun("Would install Homebrew")
		if platform.Current() == platform.Darwin {
			return "/opt/homebrew/bin/brew", nil
		}
		return "/home/linuxbrew/.linuxbrew/bin/brew", nil
	}

	fmt.Print("Homebrew is not installed. Install it now? This will download and run the official Homebrew install script. [y/N] ")
	scanner := bufio.NewScanner(stdin)
	scanner.Scan()
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if answer != "y" && answer != "yes" {
		return "", errors.New("homebrew installation declined by user")
	}

	logger.Info("Installing Homebrew...")
	brewInstallCtx, brewInstallCancel := context.WithTimeout(context.Background(), pkgInstallTimeout)
	defer brewInstallCancel()
	if err := getRunner().RunWithContext(brewInstallCtx, "/bin/bash", []string{"-c", "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"}, execx.CommandOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		return "", err
	}
	if path, err := getRunner().LookPath("brew"); err == nil {
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
	ctx, cancel := context.WithTimeout(context.Background(), pkgInstallTimeout)
	defer cancel()
	if err := runCmdContext(ctx, brewPath, "install", pkg); err == nil {
		logger.Success(fmt.Sprintf("Installed %s", pkg))
		return true
	}
	if err := runCmdContext(ctx, brewPath, "install", "--cask", pkg); err == nil {
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
	ctx, cancel := context.WithTimeout(context.Background(), pkgInstallTimeout)
	defer cancel()
	if err := runCmdContext(ctx, "sudo", "apt", "install", "-y", pkg); err != nil {
		logger.Error(fmt.Sprintf("Failed apt install for %s: %v", pkg, err))
		return false
	}
	logger.Success(fmt.Sprintf("Installed %s", pkg))
	return true
}

// fontCaskName converts a nerd font name from the config into the
// corresponding Homebrew cask name.
// Example: "JetBrainsMono" -> "font-jet-brains-mono-nerd-font"
func fontCaskName(name string) string {
	// Insert a hyphen before each uppercase letter that follows a lowercase
	// letter or digit (CamelCase -> kebab-case).
	var b strings.Builder
	runes := []rune(name)
	for i, r := range runes {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := runes[i-1]
			if (prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9') {
				b.WriteRune('-')
			}
		}
		b.WriteRune(r)
	}
	return "font-" + strings.ToLower(b.String()) + "-nerd-font"
}

// installFont installs the nerd font specified in the config using Homebrew
// casks. This eliminates the shell-injection risk of the prior manual
// download+copy approach.
func installFont(cfg config.PackageConfig, brewPath string, dryRun bool, logger *output.Logger) error {
	fontName := cfg.NerdFont
	if fontName == "" {
		fontName = "FiraCode"
	}
	cask := fontCaskName(fontName)

	// Check whether the cask is already installed.
	if brewPath != "" && runCmd(brewPath, "list", "--cask", cask) == nil {
		logger.Info(fmt.Sprintf("%s already installed", cask))
		return nil
	}

	if dryRun {
		logger.DryRun(fmt.Sprintf("Would install %s via brew --cask", cask))
		return nil
	}

	if brewPath == "" {
		return errors.New("brew not available; cannot install font")
	}

	ctx, cancel := context.WithTimeout(context.Background(), pkgInstallTimeout)
	defer cancel()
	if err := runCmdContext(ctx, brewPath, "install", "--cask", cask); err != nil {
		return fmt.Errorf("brew install --cask %s: %w", cask, err)
	}

	// Refresh the font cache so the font is immediately available.
	fontDir := filepath.Join(os.Getenv("HOME"), ".local", "share", "fonts")
	_ = runCmd("fc-cache", "-f", fontDir)

	logger.Success(fmt.Sprintf("Installed %s", cask))
	return nil
}

func runCmd(name string, args ...string) error {
	return getRunner().Run(name, args...)
}

func runCmdContext(ctx context.Context, name string, args ...string) error {
	return getRunner().RunContext(ctx, name, args...)
}
