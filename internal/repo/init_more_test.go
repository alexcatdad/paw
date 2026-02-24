package repo

import (
	"strings"
	"testing"

	"github.com/alexcatdad/paw/internal/clock"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/testutil"
)

func TestRunInitExistingRepoSameRemotePulls(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	clonePath := t.TempDir()
	logger := output.NewLogger("text", true, false)

	fake := &testutil.FakeRunner{
		OutputFn: func(name string, args ...string) ([]byte, error) {
			if strings.Contains(strings.Join(args, " "), "remote get-url origin") {
				return []byte("https://github.com/example/repo.git\n"), nil
			}
			return []byte{}, nil
		},
	}
	SetDependencies(fake, clock.RealClock{})
	t.Cleanup(func() { SetDependencies(execx.NewOSRunner(), clock.RealClock{}) })

	ok, err := RunInit("https://github.com/example/repo", InitOptions{Path: clonePath, Force: true}, logger)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected success")
	}
	if !strings.Contains(fake.JoinedCalls(), "pull --rebase") {
		t.Fatalf("expected pull call, got:\n%s", fake.JoinedCalls())
	}
}

func TestRunInitExistingRepoDifferentRemoteFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	clonePath := t.TempDir()
	logger := output.NewLogger("text", true, false)

	fake := &testutil.FakeRunner{
		OutputFn: func(name string, args ...string) ([]byte, error) {
			if strings.Contains(strings.Join(args, " "), "remote get-url origin") {
				return []byte("https://github.com/other/repo.git\n"), nil
			}
			return []byte{}, nil
		},
	}
	SetDependencies(fake, clock.RealClock{})
	t.Cleanup(func() { SetDependencies(execx.NewOSRunner(), clock.RealClock{}) })

	_, err := RunInit("https://github.com/example/repo", InitOptions{Path: clonePath, Force: true}, logger)
	if err == nil {
		t.Fatal("expected error for different remote")
	}
}
