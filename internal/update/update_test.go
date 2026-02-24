package update

import (
	"os"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	if compareVersions("1.0.0", "1.1.0") >= 0 {
		t.Fatal("expected version compare < 0")
	}
	if compareVersions("1.2.0", "1.2.0") != 0 {
		t.Fatal("expected equal")
	}
	if compareVersions("2.0.0", "1.9.9") <= 0 {
		t.Fatal("expected greater")
	}
}

func TestMapArch(t *testing.T) {
	if mapArch("amd64") != "x64" {
		t.Fatal("amd64 mapping failed")
	}
	if mapArch("arm64") != "arm64" {
		t.Fatal("arm64 mapping failed")
	}
}

func TestSaveLoadState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	in := updateState{LastCheck: "2026-01-01T00:00:00Z", LatestVersion: "1.2.3", CurrentVersion: "1.0.0"}
	if err := saveState(in); err != nil {
		t.Fatal(err)
	}
	out, err := loadState()
	if err != nil {
		t.Fatal(err)
	}
	if out == nil || out.LatestVersion != in.LatestVersion {
		t.Fatalf("unexpected state: %+v", out)
	}
	if _, err := os.Stat(home); err != nil {
		t.Fatal(err)
	}
}
