package repo

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexcatdad/paw/internal/clock"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/testutil"
)

func withFakeDeps(t *testing.T, r *testutil.FakeRunner, c clock.Clock) {
	t.Helper()
	SetDependencies(r, c)
	t.Cleanup(func() {
		SetDependencies(execx.NewOSRunner(), clock.RealClock{})
	})
}

func TestRepoStatusParsesBehindAhead(t *testing.T) {
	fake := &testutil.FakeRunner{
		CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
			if strings.Contains(strings.Join(args, " "), "rev-list") {
				return []byte("2 5\n"), nil
			}
			return []byte{}, nil
		},
	}
	withFakeDeps(t, fake, clock.RealClock{})

	behind, ahead, commits, err := RepoStatus("/tmp/repo")
	if err != nil {
		t.Fatal(err)
	}
	if !behind || !ahead || commits != 5 {
		t.Fatalf("unexpected status: behind=%v ahead=%v commits=%d", behind, ahead, commits)
	}
}

func TestPullRepoConflictReturnsNil(t *testing.T) {
	fake := &testutil.FakeRunner{
		OutputFn: func(name string, args ...string) ([]byte, error) {
			line := strings.Join(args, " ")
			switch {
			case strings.Contains(line, "rev-parse HEAD"):
				return []byte("abc123\n"), nil
			case strings.Contains(line, "status --porcelain"):
				return []byte(" M file\n"), nil
			}
			return []byte{}, nil
		},
		CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
			if strings.Contains(strings.Join(args, " "), "pull --rebase") {
				return []byte("CONFLICT (content): merge conflict\n"), errors.New("conflict")
			}
			return []byte{}, nil
		},
	}
	withFakeDeps(t, fake, clock.RealClock{})

	changed, err := PullRepo("/tmp/repo", false)
	if err != nil {
		t.Fatal(err)
	}
	if changed != nil {
		t.Fatalf("expected nil changed on conflict, got %#v", changed)
	}
	if !strings.Contains(fake.JoinedCalls(), "rebase --abort") {
		t.Fatalf("expected rebase abort, calls:\n%s", fake.JoinedCalls())
	}
	if !strings.Contains(fake.JoinedCalls(), "stash pop") {
		t.Fatalf("expected stash pop, calls:\n%s", fake.JoinedCalls())
	}
}

func TestPullRepoReturnsChangedFiles(t *testing.T) {
	fake := &testutil.FakeRunner{
		OutputFn: func(name string, args ...string) ([]byte, error) {
			line := strings.Join(args, " ")
			switch {
			case strings.Contains(line, "rev-parse HEAD"):
				if strings.Contains(line, "new") {
					return []byte("def456\n"), nil
				}
				return []byte("abc123\n"), nil
			case strings.Contains(line, "status --porcelain"):
				return []byte(""), nil
			case strings.Contains(line, "diff --name-only"):
				return []byte("home/.zshrc\nREADME.md\n"), nil
			}
			return []byte{}, nil
		},
		CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
			if strings.Contains(strings.Join(args, " "), "pull --rebase") {
				return []byte("ok"), nil
			}
			return []byte{}, nil
		},
	}
	revCount := 0
	fake.OutputFn = func(name string, args ...string) ([]byte, error) {
		line := strings.Join(args, " ")
		switch {
		case strings.Contains(line, "rev-parse HEAD"):
			revCount++
			if revCount == 1 {
				return []byte("abc123\n"), nil
			}
			return []byte("def456\n"), nil
		case strings.Contains(line, "status --porcelain"):
			return []byte(""), nil
		case strings.Contains(line, "diff --name-only"):
			return []byte("home/.zshrc\nREADME.md\n"), nil
		}
		return []byte{}, nil
	}
	withFakeDeps(t, fake, clock.RealClock{})

	changed, err := PullRepo("/tmp/repo", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 2 || changed[0] != "home/.zshrc" {
		t.Fatalf("unexpected changed files: %#v", changed)
	}
}

func TestRunPushUsesDefaultMessage(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("PAW_REPO", repoPath)
	fake := &testutil.FakeRunner{
		CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
			line := strings.Join(args, " ")
			switch {
			case strings.Contains(line, "status --porcelain"):
				return []byte(" M paw.toml\n"), nil
			case strings.Contains(line, "commit -m"):
				if !strings.Contains(line, "2026-02-24") {
					t.Fatalf("expected deterministic date in commit message, got: %s", line)
				}
				return []byte("committed"), nil
			default:
				return []byte("ok"), nil
			}
		},
	}
	withFakeDeps(t, fake, testutil.FakeClock{Instant: time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)})
	logger := output.NewLogger("text", true, false)

	if err := RunPush("", PushOptions{}, logger); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fake.JoinedCalls(), filepath.Join("git")) && len(fake.Calls) == 0 {
		t.Fatal("expected git calls")
	}
	if !strings.Contains(fake.JoinedCalls(), "push") {
		t.Fatalf("expected push call, got:\n%s", fake.JoinedCalls())
	}
}
