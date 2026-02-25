package update

import (
	"bytes"
	"errors"
	"io"
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

// TestMkdirTempUsesRandomSuffix verifies that the temp dir created during
// Perform is non-predictable: two concurrent calls must produce different paths,
// and the directory name must carry the expected "paw-update-" prefix.
func TestMkdirTempUsesRandomSuffix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var dirs []string

	makeFake := func(bin string) *testutil.FakeRunner {
		return &testutil.FakeRunner{
			LookPathFn: func(file string) (string, error) {
				if file == "paw" {
					return bin, nil
				}
				return file, nil
			},
			OutputFn: func(name string, args ...string) ([]byte, error) {
				if strings.Contains(strings.Join(args, " "), "release view latest") {
					return []byte(`{"tagName":"v2.0.0"}`), nil
				}
				// --help: no verify-attestation
				return []byte(""), nil
			},
			CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
				line := strings.Join(args, " ")
				if strings.HasPrefix(line, "release download latest") {
					dir := ""
					for i := 0; i < len(args)-1; i++ {
						if args[i] == "--dir" {
							dir = args[i+1]
						}
					}
					dirs = append(dirs, dir)
					asset := "paw-" + runtime.GOOS + "-" + mapArch(runtime.GOARCH)
					if err := os.WriteFile(filepath.Join(dir, asset), []byte("new"), 0o755); err != nil {
						t.Fatal(err)
					}
				}
				return []byte("paw version 2.0.0"), nil
			},
		}
	}

	now := testutil.FakeClock{Instant: time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)}
	logger := output.NewLogger("text", true, false)

	// Run Perform twice to collect two temp dir paths
	for i := 0; i < 2; i++ {
		bin := filepath.Join(t.TempDir(), "paw")
		if err := os.WriteFile(bin, []byte("old"), 0o755); err != nil {
			t.Fatal(err)
		}
		withDeps(t, makeFake(bin), fsx.NewOSFS(), now)
		if err := Perform("1.0.0", Options{ForceCheck: true, SkipVerify: true}, logger); err != nil {
			t.Fatalf("run %d: unexpected error: %v", i, err)
		}
	}

	if len(dirs) < 2 {
		t.Fatalf("expected 2 captured temp dirs, got %d", len(dirs))
	}

	// Both paths must carry the expected prefix.
	for _, d := range dirs {
		base := filepath.Base(d)
		if !strings.HasPrefix(base, "paw-update-") {
			t.Fatalf("expected temp dir name to start with paw-update-, got %q", base)
		}
	}

	// The two paths must differ (os.MkdirTemp randomizes the suffix; a
	// simple timestamp-based scheme reusing the same FakeClock instant
	// would produce the same path both times).
	if dirs[0] == dirs[1] {
		t.Fatalf("expected two distinct temp dirs (random suffix), both were %q", dirs[0])
	}
}

// TestAttestationWarningWhenUnavailable verifies that a clear warning is
// printed to stderr when gh does not support --verify-attestation.
func TestAttestationWarningWhenUnavailable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

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
			// download --help: does NOT contain verify-attestation
			return []byte("some other help text"), nil
		},
		CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
			line := strings.Join(args, " ")
			if strings.HasPrefix(line, "release download latest") {
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
			return []byte("paw version 2.0.0"), nil
		},
	}

	now := testutil.FakeClock{Instant: time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)}
	withDeps(t, fake, fsx.NewOSFS(), now)
	logger := output.NewLogger("text", true, false)

	// Redirect stderr to capture the warning
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w

	performErr := Perform("1.0.0", Options{ForceCheck: true}, logger)

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	stderrOutput := buf.String()

	if performErr != nil {
		t.Fatalf("unexpected error: %v", performErr)
	}
	if !strings.Contains(stderrOutput, "WARNING") {
		t.Fatalf("expected attestation warning on stderr, got: %q", stderrOutput)
	}
	if !strings.Contains(stderrOutput, "attestation") {
		t.Fatalf("expected warning to mention attestation, got: %q", stderrOutput)
	}
	// Ensure --verify-attestation was NOT passed to the download command
	for _, call := range fake.Calls {
		for _, arg := range call.Args {
			if arg == "--verify-attestation" {
				t.Fatal("--verify-attestation should not be passed when gh does not support it")
			}
		}
	}
}

// TestSkipVerifyWarning verifies that a warning is printed when SkipVerify is set.
func TestSkipVerifyWarning(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

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
			if strings.Contains(strings.Join(args, " "), "release view latest") {
				return []byte(`{"tagName":"v2.0.0"}`), nil
			}
			return []byte(""), nil
		},
		CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
			line := strings.Join(args, " ")
			if strings.HasPrefix(line, "release download latest") {
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
			return []byte("paw version 2.0.0"), nil
		},
	}

	now := testutil.FakeClock{Instant: time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)}
	withDeps(t, fake, fsx.NewOSFS(), now)
	logger := output.NewLogger("text", true, false)

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w

	performErr := Perform("1.0.0", Options{ForceCheck: true, SkipVerify: true}, logger)

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	stderrOutput := buf.String()

	if performErr != nil {
		t.Fatalf("unexpected error: %v", performErr)
	}
	if !strings.Contains(stderrOutput, "WARNING") {
		t.Fatalf("expected skip-verify warning on stderr, got: %q", stderrOutput)
	}
	if !strings.Contains(stderrOutput, "skip-verify") {
		t.Fatalf("expected warning to mention skip-verify, got: %q", stderrOutput)
	}
}

// TestStateFilePermissions verifies that the update state file is written
// with 0o600 (owner read/write only).
func TestStateFilePermissions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	st := updateState{
		LastCheck:      "2026-01-01T00:00:00Z",
		LatestVersion:  "1.2.3",
		CurrentVersion: "1.0.0",
	}
	if err := saveState(st); err != nil {
		t.Fatal(err)
	}

	// Confirm the state was saved correctly
	loaded, err := loadState()
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil || loaded.LatestVersion != "1.2.3" {
		t.Fatalf("unexpected state after save: %+v", loaded)
	}

	// Locate the state file and check its permissions
	stateFile := filepath.Join(home, ".config", "paw", "state", "update-state.json")
	info, err := os.Stat(stateFile)
	if err != nil {
		t.Fatalf("could not stat state file: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Fatalf("expected state file permissions 0o600, got %04o", perm)
	}
}

// keep errors import used by other test files in the package
var _ = errors.New
