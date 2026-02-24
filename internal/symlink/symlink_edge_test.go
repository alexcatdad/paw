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
	"github.com/alexcatdad/paw/internal/testutil"
)

type failOnNthSymlinkFS struct {
	testutil.OSBackedFS
	count int
	failN int
}

func (f *failOnNthSymlinkFS) Symlink(oldname string, newname string) error {
	f.count++
	if f.count == f.failN {
		return errors.New("forced symlink failure")
	}
	return f.OSBackedFS.Symlink(oldname, newname)
}

func TestCreateSkipsAndValidation(t *testing.T) {
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

	entries := []Entry{
		{SourceAbs: source, TargetAbs: filepath.Join(home, ".platform"), Platform: []string{"windows"}},
		{SourceAbs: source, TargetAbs: filepath.Join(home, ".host"), Hostname: "definitely-not-this-host"},
	}
	states, err := Create(entries, LinkOptions{NoInteractive: true}, logger)
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 2 {
		t.Fatalf("expected 2 skipped states, got %d", len(states))
	}
	if states[0].Status != StatusSkipped || states[1].Status != StatusSkipped {
		t.Fatalf("expected skipped states, got %#v", states)
	}

	_, err = Create([]Entry{{SourceAbs: source, TargetAbs: "/tmp/outside-home"}}, LinkOptions{NoInteractive: true}, logger)
	if err == nil {
		t.Fatal("expected validate target error")
	}
}

func TestCreateRollbackOnSymlinkError(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	t.Setenv("HOME", home)
	logger := output.NewLogger("text", true, false)

	sourceA := filepath.Join(repoDir, "home", ".a")
	sourceB := filepath.Join(repoDir, "home", ".b")
	if err := os.MkdirAll(filepath.Dir(sourceA), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourceA, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourceB, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	targetA := filepath.Join(home, ".a")
	targetB := filepath.Join(home, ".b")

	customFS := &failOnNthSymlinkFS{OSBackedFS: testutil.OSBackedFS{}, failN: 2}
	SetDependencies(runner, customFS, clk)
	t.Cleanup(func() { SetDependencies(execx.NewOSRunner(), fsx.NewOSFS(), clock.RealClock{}) })

	_, err := Create([]Entry{
		{SourceAbs: sourceA, TargetAbs: targetA},
		{SourceAbs: sourceB, TargetAbs: targetB},
	}, LinkOptions{NoInteractive: true}, logger)
	if err == nil {
		t.Fatal("expected create error")
	}
	if _, statErr := os.Lstat(targetA); !os.IsNotExist(statErr) {
		t.Fatalf("expected rollback removed first link, err=%v", statErr)
	}
}

type failWriteFS struct{ testutil.OSBackedFS }

func (failWriteFS) WriteFile(string, []byte, os.FileMode) error {
	return errors.New("forced write failure")
}

type lstatFailFS struct{ testutil.OSBackedFS }

func (lstatFailFS) Lstat(string) (os.FileInfo, error) {
	return nil, errors.New("lstat failed")
}

func TestCreateRollbackOnTransactionWriteError(t *testing.T) {
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

	SetDependencies(runner, failWriteFS{}, clk)
	t.Cleanup(func() { SetDependencies(execx.NewOSRunner(), fsx.NewOSFS(), clock.RealClock{}) })

	_, err := Create([]Entry{{SourceAbs: source, TargetAbs: target}}, LinkOptions{NoInteractive: true}, logger)
	if err == nil {
		t.Fatal("expected transaction write error")
	}
	if _, statErr := os.Lstat(target); !os.IsNotExist(statErr) {
		t.Fatalf("expected rollback removal after transaction error, err=%v", statErr)
	}
}

func TestStatusAndHelpersErrorPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	logger := output.NewLogger("text", true, false)
	t.Cleanup(func() { SetDependencies(execx.NewOSRunner(), fsx.NewOSFS(), clock.RealClock{}) })
	if err := Remove([]Entry{{TargetAbs: "/tmp/outside-home"}}, LinkOptions{}, logger); err == nil {
		t.Fatal("expected remove validate target error")
	}

	badFSPath := filepath.Join(home, "source")
	if err := os.WriteFile(badFSPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	badTarget := filepath.Join(home, "target")

	// Swap to failing fs for helper error branch only.
	SetDependencies(runner, testutil.FailingFS{}, clk)
	if _, err := fileOrLinkExists(filepath.Join(home, "x")); err == nil {
		t.Fatal("expected fileOrLinkExists error")
	}
	SetDependencies(runner, fsx.NewOSFS(), clk)
	if _, err := Status([]Entry{{SourceAbs: badFSPath, TargetAbs: badTarget}}); err != nil {
		t.Fatalf("expected status success with missing target, got %v", err)
	}

	SetDependencies(runner, lstatFailFS{}, clk)
	if _, err := Status([]Entry{{SourceAbs: badFSPath, TargetAbs: badTarget}}); err == nil {
		t.Fatal("expected status error with failing fs")
	}
}

func TestPromptConflictDiffAndInvalidChoice(t *testing.T) {
	oldStdin := os.Stdin
	file, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("dxs"); err != nil {
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

	fakeRunner := &testutil.FakeRunner{}
	SetDependencies(fakeRunner, fsx.NewOSFS(), clock.RealClock{})
	t.Cleanup(func() { SetDependencies(execx.NewOSRunner(), fsx.NewOSFS(), clock.RealClock{}) })

	choice, err := promptConflict("/tmp/target", "/tmp/source")
	if err != nil {
		t.Fatal(err)
	}
	if choice.Action != "skip" {
		t.Fatalf("expected skip after diff/invalid flow, got %+v", choice)
	}
	if len(fakeRunner.Calls) == 0 {
		t.Fatal("expected diff runner call")
	}
}
