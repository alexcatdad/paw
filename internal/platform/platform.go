package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	Darwin = "darwin"
	Linux  = "linux"
	WSL    = "wsl"
)

func Current() string {
	if runtime.GOOS == "darwin" {
		return Darwin
	}
	if runtime.GOOS == "linux" {
		if IsWSL() {
			return WSL
		}
		return Linux
	}
	return runtime.GOOS
}

func IsWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" || os.Getenv("WSL_INTEROP") != "" {
		return true
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	text := strings.ToLower(string(data))
	return strings.Contains(text, "microsoft")
}

func SystemInfo() string {
	return fmt.Sprintf("%s (%s)", Current(), runtime.GOARCH)
}

func Hostname() string {
	host, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return host
}

func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func MatchPlatform(allowed []string, current string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, item := range allowed {
		if strings.EqualFold(strings.TrimSpace(item), current) {
			return true
		}
	}
	return false
}

func MatchHostname(pattern string, hostname string) bool {
	if strings.TrimSpace(pattern) == "" || pattern == "*" {
		return true
	}
	// Use filepath.Match for proper glob semantics
	matched, err := filepath.Match(strings.ToLower(pattern), strings.ToLower(hostname))
	if err != nil {
		// Invalid pattern - fall back to exact match
		return strings.EqualFold(pattern, hostname)
	}
	return matched
}
