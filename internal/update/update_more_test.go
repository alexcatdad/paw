package update

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/alexcatdad/paw/internal/fsx"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/testutil"
)

type renameFailFS struct{ fsx.OSFS }

func (renameFailFS) Rename(string, string) error { return errors.New("rename failed") }

type chmodFailFS struct{ fsx.OSFS }

func (chmodFailFS) Chmod(string, os.FileMode) error { return errors.New("chmod failed") }

func TestCheckForUpdateCacheAndBadJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	now := time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)
	if err := saveState(updateState{
		LastCheck:      now.Format(time.RFC3339),
		LatestVersion:  "2.0.0",
		CurrentVersion: "1.0.0",
	}); err != nil {
		t.Fatal(err)
	}
	fake := &testutil.FakeRunner{
		LookPathFn: func(string) (string, error) { return "", errors.New("should not be called with cache") },
	}
	withDeps(t, fake, fsx.NewOSFS(), testutil.FakeClock{Instant: now.Add(10 * time.Minute)})
	latest, err := CheckForUpdate("1.0.0", false)
	if err != nil {
		t.Fatal(err)
	}
	if latest != "2.0.0" {
		t.Fatalf("expected cached latest version, got %q", latest)
	}
	none, err := CheckForUpdate("2.0.0", false)
	if err != nil {
		t.Fatal(err)
	}
	if none != "" {
		t.Fatalf("expected no update, got %q", none)
	}

	badJSON := &testutil.FakeRunner{
		OutputFn: func(name string, args ...string) ([]byte, error) {
			return []byte("{not-json"), nil
		},
	}
	withDeps(t, badJSON, fsx.NewOSFS(), testutil.FakeClock{Instant: now})
	if _, err := CheckForUpdate("1.0.0", true); err == nil {
		t.Fatal("expected JSON parse error")
	}
}

func TestPerformErrorBranches(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	logger := output.NewLogger("text", true, false)
	now := testutil.FakeClock{Instant: time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)}

	t.Run("no update available", func(t *testing.T) {
		fake := &testutil.FakeRunner{
			OutputFn: func(name string, args ...string) ([]byte, error) {
				if strings.Contains(strings.Join(args, " "), "release view latest") {
					return []byte(`{"tagName":"v1.0.0"}`), nil
				}
				return []byte{}, nil
			},
		}
		withDeps(t, fake, fsx.NewOSFS(), now)
		if err := Perform("1.0.0", Options{ForceCheck: true}, logger); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("paw binary missing", func(t *testing.T) {
		fake := &testutil.FakeRunner{
			LookPathFn: func(file string) (string, error) {
				if file == "paw" {
					return "", errors.New("missing paw")
				}
				return file, nil
			},
			OutputFn: func(name string, args ...string) ([]byte, error) {
				return []byte(`{"tagName":"v2.0.0"}`), nil
			},
		}
		withDeps(t, fake, fsx.NewOSFS(), now)
		if err := Perform("1.0.0", Options{ForceCheck: true}, logger); err == nil {
			t.Fatal("expected missing paw error")
		}
	})

	t.Run("download failure", func(t *testing.T) {
		bin := filepath.Join(t.TempDir(), "paw")
		if err := os.WriteFile(bin, []byte("old"), 0o755); err != nil {
			t.Fatal(err)
		}
		fake := &testutil.FakeRunner{
			LookPathFn: func(file string) (string, error) {
				if file == "paw" {
					return bin, nil
				}
				return file, nil
			},
			OutputFn: func(name string, args ...string) ([]byte, error) {
				line := strings.Join(args, " ")
				if strings.Contains(line, "release view latest") {
					return []byte(`{"tagName":"v2.0.0"}`), nil
				}
				if strings.Contains(line, "download --help") {
					return []byte("verify-attestation"), nil
				}
				return []byte{}, nil
			},
			CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
				if strings.HasPrefix(strings.Join(args, " "), "release download latest") {
					return []byte("download failed"), errors.New("download fail")
				}
				return []byte{}, nil
			},
		}
		withDeps(t, fake, fsx.NewOSFS(), now)
		if err := Perform("1.0.0", Options{ForceCheck: true}, logger); err == nil {
			t.Fatal("expected download error")
		}
	})

	t.Run("rename failure", func(t *testing.T) {
		bin := filepath.Join(t.TempDir(), "paw")
		if err := os.WriteFile(bin, []byte("old"), 0o755); err != nil {
			t.Fatal(err)
		}
		fake := &testutil.FakeRunner{
			LookPathFn: func(file string) (string, error) {
				if file == "paw" {
					return bin, nil
				}
				return file, nil
			},
			OutputFn: func(name string, args ...string) ([]byte, error) {
				line := strings.Join(args, " ")
				if strings.Contains(line, "release view latest") {
					return []byte(`{"tagName":"v2.0.0"}`), nil
				}
				if strings.Contains(line, "download --help") {
					return []byte(""), nil
				}
				return []byte{}, nil
			},
			CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
				if strings.HasPrefix(strings.Join(args, " "), "release download latest") {
					dir := ""
					for i := 0; i < len(args)-1; i++ {
						if args[i] == "--dir" {
							dir = args[i+1]
						}
					}
					if dir == "" {
						t.Fatal("expected --dir argument")
					}
					asset := "paw-" + runtime.GOOS + "-" + mapArch(runtime.GOARCH)
					if err := os.WriteFile(filepath.Join(dir, asset), []byte("new"), 0o755); err != nil {
						t.Fatal(err)
					}
				}
				return []byte{}, nil
			},
		}
		withDeps(t, fake, renameFailFS{}, now)
		if err := Perform("1.0.0", Options{ForceCheck: true, SkipVerify: true}, logger); err == nil {
			t.Fatal("expected rename error")
		}
	})

	t.Run("chmod failure", func(t *testing.T) {
		bin := filepath.Join(t.TempDir(), "paw")
		if err := os.WriteFile(bin, []byte("old"), 0o755); err != nil {
			t.Fatal(err)
		}
		fake := &testutil.FakeRunner{
			LookPathFn: func(file string) (string, error) {
				if file == "paw" {
					return bin, nil
				}
				return file, nil
			},
			OutputFn: func(name string, args ...string) ([]byte, error) {
				line := strings.Join(args, " ")
				if strings.Contains(line, "release view latest") {
					return []byte(`{"tagName":"v2.0.0"}`), nil
				}
				if strings.Contains(line, "download --help") {
					return []byte(""), nil
				}
				return []byte{}, nil
			},
			CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
				if strings.HasPrefix(strings.Join(args, " "), "release download latest") {
					dir := ""
					for i := 0; i < len(args)-1; i++ {
						if args[i] == "--dir" {
							dir = args[i+1]
						}
					}
					asset := "paw-" + runtime.GOOS + "-" + mapArch(runtime.GOARCH)
					if err := os.WriteFile(filepath.Join(dir, asset), []byte("new"), 0o755); err != nil {
						t.Fatal(err)
					}
				}
				if name == bin {
					return []byte("ok"), nil
				}
				return []byte{}, nil
			},
		}
		withDeps(t, fake, chmodFailFS{}, now)
		if err := Perform("1.0.0", Options{ForceCheck: true, SkipVerify: true}, logger); err == nil {
			t.Fatal("expected chmod error")
		}
	})
}
