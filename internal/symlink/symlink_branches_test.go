package symlink

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/alexcatdad/paw/internal/clock"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/fsx"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/state"
	"github.com/alexcatdad/paw/internal/testutil"
)

type removeFailFS struct{ testutil.OSBackedFS }

func (removeFailFS) Remove(string) error { return errors.New("forced remove failure") }

type readlinkFailFS struct{ testutil.OSBackedFS }

func (readlinkFailFS) Readlink(string) (string, error) {
	return "", errors.New("forced readlink failure")
}

type lstatErrOnlyFS struct{ testutil.OSBackedFS }

func (lstatErrOnlyFS) Lstat(string) (os.FileInfo, error) {
	return nil, errors.New("forced lstat failure")
}

func TestPromptConflictChoiceVariants(t *testing.T) {
	cases := []struct {
		input      string
		wantAction string
		wantAll    bool
	}{
		{input: "b", wantAction: "backup"},
		{input: "o", wantAction: "overwrite"},
		{input: "a", wantAction: "abort"},
		{input: "S", wantAction: "skip", wantAll: true},
		{input: "B", wantAction: "backup", wantAll: true},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			oldStdin := os.Stdin
			file, err := os.CreateTemp(t.TempDir(), "stdin")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := file.WriteString(tc.input); err != nil {
				t.Fatal(err)
			}
			if _, err := file.Seek(0, 0); err != nil {
				t.Fatal(err)
			}
			os.Stdin = file
			t.Cleanup(func() {
				os.Stdin = oldStdin
				_ = file.Close()
			})

			choice, err := promptConflict("/tmp/target", "/tmp/source")
			if err != nil {
				t.Fatal(err)
			}
			if choice.Action != tc.wantAction || choice.ApplyToAll != tc.wantAll {
				t.Fatalf("unexpected choice: %+v", choice)
			}
		})
	}
}

func TestRemoveAndStatusAdditionalBranches(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	t.Setenv("HOME", home)
	logger := output.NewLogger("text", true, false)

	source := filepath.Join(repoDir, "home", ".zshrc")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, ".zshrc")
	if err := os.Symlink(source, target); err != nil {
		t.Fatal(err)
	}

	if err := Remove([]Entry{{SourceAbs: source, TargetAbs: target}}, LinkOptions{DryRun: true}, logger); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(target); err != nil {
		t.Fatalf("expected dry-run to keep link: %v", err)
	}

	SetDependencies(runner, removeFailFS{}, clk)
	t.Cleanup(func() { SetDependencies(execx.NewOSRunner(), fsx.NewOSFS(), clock.RealClock{}) })
	if err := Remove([]Entry{{SourceAbs: source, TargetAbs: target}}, LinkOptions{}, logger); err == nil {
		t.Fatal("expected remove error")
	}

	conflictTarget := filepath.Join(home, ".conflict")
	if err := os.WriteFile(conflictTarget, []byte("not a link"), 0o644); err != nil {
		t.Fatal(err)
	}
	states, err := Status([]Entry{
		{SourceAbs: source, TargetAbs: filepath.Join(home, ".skip"), Hostname: "definitely-not-this-host"},
		{SourceAbs: source, TargetAbs: conflictTarget},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 2 {
		t.Fatalf("expected 2 states, got %d", len(states))
	}
	if states[0].Status != StatusSkipped || states[1].Status != StatusConflict {
		t.Fatalf("unexpected states: %+v", states)
	}
}

func TestHelperErrorBranches(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	src := filepath.Join(home, "src")
	dst := filepath.Join(home, "dst")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("src", dst); err != nil {
		t.Fatal(err)
	}

	SetDependencies(runner, lstatErrOnlyFS{}, clk)
	if _, err := isSymlinkTo(dst, src); err == nil {
		t.Fatal("expected lstat error")
	}

	SetDependencies(runner, readlinkFailFS{}, clk)
	if _, err := isSymlinkTo(dst, src); err == nil {
		t.Fatal("expected readlink error")
	}

	SetDependencies(runner, fsx.NewOSFS(), clk)
	ok, err := isSymlinkTo(dst, src)
	if err != nil || !ok {
		t.Fatalf("expected relative symlink resolution, ok=%v err=%v", ok, err)
	}

	if !isIgnored(".zshrc", []string{"", "~/.zshrc"}) {
		t.Fatal("expected ignore handling for empty and normalized entries")
	}

	oldStdin := os.Stdin
	closed, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	_ = closed.Close()
	os.Stdin = closed
	t.Cleanup(func() {
		os.Stdin = oldStdin
		SetDependencies(execx.NewOSRunner(), fsx.NewOSFS(), clock.RealClock{})
	})
	if isInteractive() {
		t.Fatal("expected non-interactive when stdin stat fails")
	}
}

func TestLockAndTransactionErrorBranches(t *testing.T) {
	homeRoot := t.TempDir()
	home := filepath.Join(homeRoot, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	lockPath, err := state.LockPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, []byte("locked"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := acquireLock(); err == nil {
		t.Fatal("expected lock contention error")
	}

	homeFile := filepath.Join(homeRoot, "home-file")
	if err := os.WriteFile(homeFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeFile)
	if err := saveTransaction(transaction{}); err == nil {
		t.Fatal("expected saveTransaction path error")
	}
}
