package platform

import (
	"runtime"
	"strings"
	"testing"
)

func TestCurrentAndSystemInfo(t *testing.T) {
	current := Current()
	if current == "" {
		t.Fatal("expected current platform")
	}
	switch runtime.GOOS {
	case "darwin":
		if current != Darwin {
			t.Fatalf("expected darwin, got %s", current)
		}
	case "linux":
		if current != Linux && current != WSL {
			t.Fatalf("expected linux/wsl, got %s", current)
		}
	default:
		if current != runtime.GOOS {
			t.Fatalf("expected %s got %s", runtime.GOOS, current)
		}
	}
	info := SystemInfo()
	if !strings.Contains(info, runtime.GOARCH) {
		t.Fatalf("expected arch in system info: %s", info)
	}
}

func TestIsWSLAndHostnameAndCommandExists(t *testing.T) {
	t.Setenv("WSL_DISTRO_NAME", "Ubuntu")
	if !IsWSL() {
		t.Fatal("expected wsl true with WSL_DISTRO_NAME")
	}
	t.Setenv("WSL_DISTRO_NAME", "")
	t.Setenv("WSL_INTEROP", "interop")
	if !IsWSL() {
		t.Fatal("expected wsl true with WSL_INTEROP")
	}
	t.Setenv("WSL_INTEROP", "")

	host := Hostname()
	if strings.TrimSpace(host) == "" {
		t.Fatal("expected non-empty hostname")
	}
	if !CommandExists("sh") {
		t.Fatal("expected sh command")
	}
	if CommandExists("definitely-missing-command-xyz") {
		t.Fatal("expected missing command")
	}
}
