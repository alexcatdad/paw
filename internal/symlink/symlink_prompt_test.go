package symlink

import (
	"os"
	"testing"

	"github.com/alexcatdad/paw/internal/clock"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/fsx"
	"github.com/alexcatdad/paw/internal/testutil"
)

func TestSetDependenciesAndShowDiff(t *testing.T) {
	fake := &testutil.FakeRunner{}
	SetDependencies(fake, fsx.NewOSFS(), testutil.FakeClock{})
	t.Cleanup(func() { SetDependencies(execx.NewOSRunner(), fsx.NewOSFS(), clock.RealClock{}) })
	showDiff("a", "b")
	if len(fake.Calls) == 0 {
		t.Fatal("expected diff command call")
	}
}

func TestPromptConflictAndIsInteractive(t *testing.T) {
	old := os.Stdin
	file, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("s"); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	os.Stdin = file
	t.Cleanup(func() {
		os.Stdin = old
		_ = file.Close()
	})

	choice, err := promptConflict("/tmp/target", "/tmp/source")
	if err != nil {
		t.Fatal(err)
	}
	if choice.Action != "skip" {
		t.Fatalf("expected skip choice, got %#v", choice)
	}
	_ = isInteractive()
}

func TestPromptConflictBackupAllChoice(t *testing.T) {
	old := os.Stdin
	file, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("B"); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	os.Stdin = file
	t.Cleanup(func() {
		os.Stdin = old
		_ = file.Close()
	})

	choice, err := promptConflict("/tmp/target", "/tmp/source")
	if err != nil {
		t.Fatal(err)
	}
	if choice.Action != "backup" || !choice.ApplyToAll {
		t.Fatalf("unexpected choice: %#v", choice)
	}
}
