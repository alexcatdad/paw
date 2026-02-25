package testutil

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/alexcatdad/paw/internal/clock"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/fsx"
)

type FakeClock struct {
	Instant time.Time
}

func (f FakeClock) Now() time.Time {
	if f.Instant.IsZero() {
		return time.Unix(0, 0).UTC()
	}
	return f.Instant
}

var _ clock.Clock = FakeClock{}

type CommandCall struct {
	Name string
	Args []string
}

type FakeRunner struct {
	LookPathFn       func(file string) (string, error)
	RunFn            func(name string, args ...string) error
	OutputFn         func(name string, args ...string) ([]byte, error)
	CombinedOutputFn func(name string, args ...string) ([]byte, error)
	RunWithFn        func(name string, args []string, opts execx.CommandOptions) error

	mu    sync.Mutex
	Calls []CommandCall
}

func (f *FakeRunner) record(name string, args ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	copied := make([]string, len(args))
	copy(copied, args)
	f.Calls = append(f.Calls, CommandCall{Name: name, Args: copied})
}

func (f *FakeRunner) LookPath(file string) (string, error) {
	if f.LookPathFn != nil {
		return f.LookPathFn(file)
	}
	return file, nil
}

func (f *FakeRunner) Run(name string, args ...string) error {
	f.record(name, args...)
	if f.RunFn != nil {
		return f.RunFn(name, args...)
	}
	return nil
}

// RunContext delegates to Run; timeouts are irrelevant in tests.
func (f *FakeRunner) RunContext(_ context.Context, name string, args ...string) error {
	return f.Run(name, args...)
}

func (f *FakeRunner) Output(name string, args ...string) ([]byte, error) {
	f.record(name, args...)
	if f.OutputFn != nil {
		return f.OutputFn(name, args...)
	}
	return []byte{}, nil
}

// OutputContext delegates to Output; timeouts are irrelevant in tests.
func (f *FakeRunner) OutputContext(_ context.Context, name string, args ...string) ([]byte, error) {
	return f.Output(name, args...)
}

func (f *FakeRunner) CombinedOutput(name string, args ...string) ([]byte, error) {
	f.record(name, args...)
	if f.CombinedOutputFn != nil {
		return f.CombinedOutputFn(name, args...)
	}
	return []byte{}, nil
}

// CombinedOutputContext delegates to CombinedOutput; timeouts are irrelevant in tests.
func (f *FakeRunner) CombinedOutputContext(_ context.Context, name string, args ...string) ([]byte, error) {
	return f.CombinedOutput(name, args...)
}

func (f *FakeRunner) RunWith(name string, args []string, opts execx.CommandOptions) error {
	f.record(name, args...)
	if f.RunWithFn != nil {
		return f.RunWithFn(name, args, opts)
	}
	return nil
}

// RunWithContext delegates to RunWith; timeouts are irrelevant in tests.
func (f *FakeRunner) RunWithContext(_ context.Context, name string, args []string, opts execx.CommandOptions) error {
	return f.RunWith(name, args, opts)
}

func (f *FakeRunner) JoinedCalls() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	lines := make([]string, 0, len(f.Calls))
	for _, c := range f.Calls {
		lines = append(lines, c.Name+" "+strings.Join(c.Args, " "))
	}
	return strings.Join(lines, "\n")
}

var _ execx.Runner = (*FakeRunner)(nil)

type OSBackedFS struct{}

func (OSBackedFS) ReadFile(name string) ([]byte, error) { return os.ReadFile(name) }
func (OSBackedFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}
func (OSBackedFS) ReadDir(name string) ([]os.DirEntry, error)   { return os.ReadDir(name) }
func (OSBackedFS) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
func (OSBackedFS) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}
func (OSBackedFS) Stat(name string) (os.FileInfo, error)        { return os.Stat(name) }
func (OSBackedFS) Lstat(name string) (os.FileInfo, error)       { return os.Lstat(name) }
func (OSBackedFS) Readlink(name string) (string, error)         { return os.Readlink(name) }
func (OSBackedFS) Symlink(oldname string, newname string) error { return os.Symlink(oldname, newname) }
func (OSBackedFS) Remove(name string) error                     { return os.Remove(name) }
func (OSBackedFS) RemoveAll(path string) error                  { return os.RemoveAll(path) }
func (OSBackedFS) Rename(oldpath string, newpath string) error  { return os.Rename(oldpath, newpath) }
func (OSBackedFS) Chmod(name string, mode os.FileMode) error    { return os.Chmod(name, mode) }

var _ fsx.FS = OSBackedFS{}

type FailingFS struct {
	Err error
}

func (f FailingFS) withErr() error {
	if f.Err != nil {
		return f.Err
	}
	return errors.New("forced fs failure")
}

func (f FailingFS) ReadFile(string) ([]byte, error)                     { return nil, f.withErr() }
func (f FailingFS) WriteFile(string, []byte, os.FileMode) error         { return f.withErr() }
func (f FailingFS) ReadDir(string) ([]os.DirEntry, error)               { return nil, f.withErr() }
func (f FailingFS) MkdirAll(string, os.FileMode) error                  { return f.withErr() }
func (f FailingFS) OpenFile(string, int, os.FileMode) (*os.File, error) { return nil, f.withErr() }
func (f FailingFS) Stat(string) (os.FileInfo, error)                    { return nil, f.withErr() }
func (f FailingFS) Lstat(string) (os.FileInfo, error)                   { return nil, f.withErr() }
func (f FailingFS) Readlink(string) (string, error)                     { return "", f.withErr() }
func (f FailingFS) Symlink(string, string) error                        { return f.withErr() }
func (f FailingFS) Remove(string) error                                 { return f.withErr() }
func (f FailingFS) RemoveAll(string) error                              { return f.withErr() }
func (f FailingFS) Rename(string, string) error                         { return f.withErr() }
func (f FailingFS) Chmod(string, os.FileMode) error                     { return f.withErr() }

var _ fsx.FS = FailingFS{}

type NopWriter struct{}

func (NopWriter) Write(p []byte) (int, error) { return len(p), nil }

var _ io.Writer = NopWriter{}
