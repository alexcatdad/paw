package repo

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alexcatdad/paw/internal/clock"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/testutil"
)

func TestRunPushBranches(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PAW_REPO", t.TempDir())
	logger := output.NewLogger("text", true, false)

	t.Run("no changes", func(t *testing.T) {
		fake := &testutil.FakeRunner{
			CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
				if strings.Contains(strings.Join(args, " "), "status --porcelain") {
					return []byte(""), nil
				}
				return []byte("ok"), nil
			},
		}
		SetDependencies(fake, clock.RealClock{})
		defer SetDependencies(execx.NewOSRunner(), clock.RealClock{})
		if err := RunPush("msg", PushOptions{}, logger); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("dry run", func(t *testing.T) {
		fake := &testutil.FakeRunner{
			CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
				return []byte(" M paw.toml"), nil
			},
		}
		SetDependencies(fake, testutil.FakeClock{Instant: time.Date(2026, 2, 24, 0, 0, 0, 0, time.UTC)})
		defer SetDependencies(execx.NewOSRunner(), clock.RealClock{})
		if err := RunPush("", PushOptions{DryRun: true, Verbose: true}, logger); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("commit nothing to commit", func(t *testing.T) {
		fake := &testutil.FakeRunner{
			CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
				line := strings.Join(args, " ")
				switch {
				case strings.Contains(line, "status --porcelain"):
					return []byte(" M paw.toml"), nil
				case strings.Contains(line, "add -A"):
					return []byte(""), nil
				case strings.Contains(line, "commit -m"):
					return []byte("nothing to commit"), errors.New("nothing")
				default:
					return []byte(""), nil
				}
			},
		}
		SetDependencies(fake, clock.RealClock{})
		defer SetDependencies(execx.NewOSRunner(), clock.RealClock{})
		if err := RunPush("msg", PushOptions{}, logger); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("add fail", func(t *testing.T) {
		fake := &testutil.FakeRunner{
			CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
				line := strings.Join(args, " ")
				switch {
				case strings.Contains(line, "status --porcelain"):
					return []byte(" M paw.toml"), nil
				case strings.Contains(line, "add -A"):
					return []byte("add failed"), errors.New("fail")
				default:
					return []byte(""), nil
				}
			},
		}
		SetDependencies(fake, clock.RealClock{})
		defer SetDependencies(execx.NewOSRunner(), clock.RealClock{})
		if err := RunPush("msg", PushOptions{}, logger); err == nil {
			t.Fatal("expected add error")
		}
	})

	t.Run("commit fail", func(t *testing.T) {
		fake := &testutil.FakeRunner{
			CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
				line := strings.Join(args, " ")
				switch {
				case strings.Contains(line, "status --porcelain"):
					return []byte(" M paw.toml"), nil
				case strings.Contains(line, "add -A"):
					return []byte(""), nil
				case strings.Contains(line, "commit -m"):
					return []byte("commit failed"), errors.New("fail")
				default:
					return []byte(""), nil
				}
			},
		}
		SetDependencies(fake, clock.RealClock{})
		defer SetDependencies(execx.NewOSRunner(), clock.RealClock{})
		if err := RunPush("msg", PushOptions{}, logger); err == nil {
			t.Fatal("expected commit error")
		}
	})

	t.Run("push fail", func(t *testing.T) {
		fake := &testutil.FakeRunner{
			CombinedOutputFn: func(name string, args ...string) ([]byte, error) {
				line := strings.Join(args, " ")
				switch {
				case strings.Contains(line, "status --porcelain"):
					return []byte(" M paw.toml"), nil
				case strings.Contains(line, "add -A"):
					return []byte(""), nil
				case strings.Contains(line, "commit -m"):
					return []byte(""), nil
				case strings.HasSuffix(line, "push"):
					return []byte("push failed"), errors.New("fail")
				default:
					return []byte(""), nil
				}
			},
		}
		SetDependencies(fake, clock.RealClock{})
		defer SetDependencies(execx.NewOSRunner(), clock.RealClock{})
		if err := RunPush("msg", PushOptions{}, logger); err == nil {
			t.Fatal("expected push error")
		}
	})
}
