package update

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/alexcatdad/paw/internal/clock"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/fsx"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/testutil"
)

func withDeps(t *testing.T, r *testutil.FakeRunner, f fsx.FS, c clock.Clock) {
	t.Helper()
	SetDependencies(r, f, c)
	t.Cleanup(func() {
		SetDependencies(execx.NewOSRunner(), fsx.NewOSFS(), clock.RealClock{})
	})
}

func TestCheckForUpdateGhMissing(t *testing.T) {
	fake := &testutil.FakeRunner{
		LookPathFn: func(file string) (string, error) {
			if file == "gh" {
				return "", errors.New("missing")
			}
			return file, nil
		},
	}
	withDeps(t, fake, fsx.NewOSFS(), clock.RealClock{})
	latest, err := CheckForUpdate("1.0.0", true)
	if err != nil {
		t.Fatal(err)
	}
	if latest != "" {
		t.Fatalf("expected no update, got %q", latest)
	}
}

func TestCheckForUpdateReturnsLatest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fake := &testutil.FakeRunner{
		OutputFn: func(name string, args ...string) ([]byte, error) {
			return []byte(`{"tagName":"v1.2.3"}`), nil
		},
	}
	withDeps(t, fake, fsx.NewOSFS(), testutil.FakeClock{Instant: time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)})
	latest, err := CheckForUpdate("1.0.0", true)
	if err != nil {
		t.Fatal(err)
	}
	if latest != "1.2.3" {
		t.Fatalf("expected 1.2.3, got %q", latest)
	}
}

func TestPerformDryRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fake := &testutil.FakeRunner{
		LookPathFn: func(file string) (string, error) {
			if file == "paw" {
				return "/usr/local/bin/paw", nil
			}
			return file, nil
		},
		OutputFn: func(name string, args ...string) ([]byte, error) {
			if strings.Join(args, " ") == "release view latest --repo alexcatdad/paw --json tagName" {
				return []byte(`{"tagName":"v2.0.0"}`), nil
			}
			return []byte(""), nil
		},
	}
	withDeps(t, fake, fsx.NewOSFS(), testutil.FakeClock{Instant: time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)})
	logger := output.NewLogger("text", true, false)
	if err := Perform("1.0.0", Options{DryRun: true, ForceCheck: true}, logger); err != nil {
		t.Fatal(err)
	}
}

func TestPerformRestoresBackupOnVerificationFailure(t *testing.T) {
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(binDir, "paw")
	if err := os.WriteFile(binPath, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	fake := &testutil.FakeRunner{}
	fake.LookPathFn = func(file string) (string, error) {
		switch file {
		case "gh":
			return "gh", nil
		case "paw":
			return binPath, nil
		default:
			return file, nil
		}
	}
	fake.OutputFn = func(name string, args ...string) ([]byte, error) {
		line := strings.Join(args, " ")
		switch {
		case strings.Contains(line, "release view latest"):
			return []byte(`{"tagName":"v2.0.0"}`), nil
		case strings.Contains(line, "release download --help"):
			return []byte("verify-attestation"), nil
		default:
			return []byte(""), nil
		}
	}
	fake.CombinedOutputFn = func(name string, args ...string) ([]byte, error) {
		line := strings.Join(args, " ")
		switch {
		case strings.HasPrefix(line, "release download latest"):
			dir := ""
			for i := 0; i < len(args)-1; i++ {
				if args[i] == "--dir" {
					dir = args[i+1]
					break
				}
			}
			if dir == "" {
				t.Fatal("missing --dir in download args")
			}
			asset := "paw-" + runtime.GOOS + "-" + mapArch(runtime.GOARCH)
			if err := os.WriteFile(filepath.Join(dir, asset), []byte("new-binary"), 0o755); err != nil {
				t.Fatal(err)
			}
			return []byte("ok"), nil
		case name == binPath:
			return []byte("bad binary"), errors.New("verification failed")
		default:
			return []byte("ok"), nil
		}
	}

	withDeps(t, fake, fsx.NewOSFS(), testutil.FakeClock{Instant: time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)})
	logger := output.NewLogger("text", true, false)
	err := Perform("1.0.0", Options{ForceCheck: true}, logger)
	if err == nil {
		t.Fatal("expected verification failure")
	}
	content, readErr := os.ReadFile(binPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(content) != "old-binary" {
		t.Fatalf("expected rollback restore, got %q", string(content))
	}
}
