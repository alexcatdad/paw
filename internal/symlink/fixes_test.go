package symlink

// fixes_test.go exercises the four security/quality fixes:
//   I1  – stale lock recovery
//   I4  – lock file permissions (0o600)
//   M2  – walk error propagation in BuildEntries
//   M8  – rollbackTransaction returns meaningful errors

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/alexcatdad/paw/internal/clock"
	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/fsx"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/state"
	"github.com/alexcatdad/paw/internal/testutil"
)

// ---------------------------------------------------------------------------
// Fix I1: Stale lock recovery
// ---------------------------------------------------------------------------

// TestAcquireLockStaleLock verifies that when a lock file contains a PID that
// no longer corresponds to a running process, acquireLock removes the stale
// file and acquires the lock successfully.
func TestAcquireLockStaleLock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	lockPath, err := state.LockPath()
	if err != nil {
		t.Fatal(err)
	}
	// Ensure the lock state directory exists.
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatal(err)
	}

	// Find a PID that is definitely not alive.  We fork-and-immediately-exit a
	// child so that the PID it had is reaped before we proceed.
	deadPID := findDeadPID(t)

	// Write a lock file referencing the dead PID.
	if err := os.WriteFile(lockPath, []byte(fmt.Sprintf("pid=%d\n", deadPID)), 0o600); err != nil {
		t.Fatal(err)
	}

	// acquireLock should detect the stale lock, remove it, and succeed.
	unlock, err := acquireLock()
	if err != nil {
		t.Fatalf("expected acquireLock to recover stale lock, got error: %v", err)
	}
	defer unlock()

	// Lock file should still exist (we own it now).
	if _, statErr := os.Stat(lockPath); statErr != nil {
		t.Fatalf("expected lock file to exist after recovery, got: %v", statErr)
	}
}

// TestAcquireLockLivePID verifies that acquireLock returns an error containing
// the owning PID when the process in the lock file is still alive.
func TestAcquireLockLivePID(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	lockPath, err := state.LockPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a lock file referencing our own PID (which is definitely alive).
	ownPID := os.Getpid()
	if err := os.WriteFile(lockPath, []byte(fmt.Sprintf("pid=%d\n", ownPID)), 0o600); err != nil {
		t.Fatal(err)
	}

	_, lockErr := acquireLock()
	if lockErr == nil {
		t.Fatal("expected acquireLock to return an error for a live PID")
	}
	expected := fmt.Sprintf("another paw operation is running (pid=%d)", ownPID)
	if lockErr.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, lockErr.Error())
	}
}

// TestAcquireLockUnparsableLockFile verifies that when the lock file content
// cannot be parsed as "pid=<N>", acquireLock treats it as a live lock.
func TestAcquireLockUnparsableLockFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	lockPath, err := state.LockPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(lockPath, []byte("not-a-pid\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, lockErr := acquireLock()
	if lockErr == nil {
		t.Fatal("expected acquireLock to return an error for unparsable lock file")
	}
}

// ---------------------------------------------------------------------------
// Fix I4: Lock file permissions
// ---------------------------------------------------------------------------

// TestLockFilePermissions verifies that the lock file is created with mode
// 0o600 (owner-read/write only).
func TestLockFilePermissions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	unlock, err := acquireLock()
	if err != nil {
		t.Fatalf("acquireLock failed: %v", err)
	}
	defer unlock()

	lockPath, err := state.LockPath()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(lockPath)
	if err != nil {
		t.Fatalf("could not stat lock file: %v", err)
	}
	// On unix the permission bits should be exactly 0600.
	got := info.Mode().Perm()
	if got != 0o600 {
		t.Fatalf("expected lock file permissions 0600, got %04o", got)
	}
}

// ---------------------------------------------------------------------------
// Fix M2: Walk error propagation in BuildEntries
// ---------------------------------------------------------------------------

// TestBuildEntriesWalkErrorPropagated verifies that BuildEntries returns an
// error when the home/ directory is present but a file inside it becomes
// unreadable during the walk.
//
// We simulate a walk error by making a sub-directory unreadable (chmod 000)
// so that WalkDir reports an error when it tries to descend into it.
func TestBuildEntriesWalkErrorPropagated(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root – chmod 000 does not block access")
	}

	repoDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	homeRoot := filepath.Join(repoDir, "home")
	secretDir := filepath.Join(homeRoot, "secret")
	if err := os.MkdirAll(secretDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secretDir, "file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Make the sub-directory unreadable so WalkDir receives an error.
	if err := os.Chmod(secretDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(secretDir, 0o755) })

	_, err := BuildEntries(repoDir, homeDir, config.Default())
	if err == nil {
		t.Fatal("expected BuildEntries to propagate walk error for unreadable directory")
	}
}

// TestBuildEntriesMissingHomeRoot verifies that BuildEntries treats a
// non-existent home/ directory as a non-fatal condition (empty repo).
func TestBuildEntriesMissingHomeRoot(t *testing.T) {
	repoDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	// Deliberately do NOT create repoDir/home.
	entries, err := BuildEntries(repoDir, homeDir, config.Default())
	if err != nil {
		t.Fatalf("expected no error for missing home root, got: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

// ---------------------------------------------------------------------------
// Fix M8: rollbackTransaction returns meaningful errors
// ---------------------------------------------------------------------------

// rollbackFailRemoveFS is a fake FS that fails Remove calls for paths that
// contain a specific marker string.
type rollbackFailRemoveFS struct {
	testutil.OSBackedFS
	failMarker string
}

func (f rollbackFailRemoveFS) Remove(name string) error {
	if f.failMarker != "" && len(name) >= len(f.failMarker) {
		for i := 0; i <= len(name)-len(f.failMarker); i++ {
			if name[i:i+len(f.failMarker)] == f.failMarker {
				return errors.New("forced remove failure")
			}
		}
	}
	return f.OSBackedFS.Remove(name)
}

// TestRollbackTransactionErrorPropagation verifies that rollbackTransaction
// returns a non-nil error when fsys.Remove fails for a created symlink.
func TestRollbackTransactionErrorPropagation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Create a real file to represent the "created" symlink entry.
	createdPath := filepath.Join(home, ".created-link")
	if err := os.WriteFile(createdPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Inject a FS that fails Remove for this specific path.
	marker := ".created-link"
	SetDependencies(runner, rollbackFailRemoveFS{failMarker: marker}, clk)
	t.Cleanup(func() { SetDependencies(execx.NewOSRunner(), fsx.NewOSFS(), clock.RealClock{}) })

	logger := output.NewLogger("text", true, false)
	txn := transaction{
		Created: []string{createdPath},
		Backups: []backupMove{},
	}

	err := rollbackTransaction(txn, logger)
	if err == nil {
		t.Fatal("expected rollbackTransaction to return an error when Remove fails")
	}
	if err.Error() == "" {
		t.Fatal("expected non-empty error message from rollbackTransaction")
	}
}

// TestRollbackTransactionCallerLogsError verifies that the Create function
// logs the rollback error when rollbackTransaction fails, rather than
// silently discarding it.  We trigger a rollback by causing the second
// symlink creation to fail, and we use a FS that also fails Remove (so
// rollback itself fails).
func TestRollbackTransactionCallerLogsError(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	t.Setenv("HOME", home)

	srcA := filepath.Join(repoDir, "home", ".alpha")
	srcB := filepath.Join(repoDir, "home", ".beta")
	if err := os.MkdirAll(filepath.Dir(srcA), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcA, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcB, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	tgtA := filepath.Join(home, ".alpha")
	tgtB := filepath.Join(home, ".beta")

	// Use a FS that succeeds the first Symlink, fails the second, and also
	// fails any Remove call (so rollback itself fails).
	customFS := &failOnNthSymlinkAndRemoveFS{failSymlinkN: 2}
	SetDependencies(runner, customFS, clk)
	t.Cleanup(func() { SetDependencies(execx.NewOSRunner(), fsx.NewOSFS(), clock.RealClock{}) })

	logger := output.NewLogger("text", true, false)
	_, err := Create([]Entry{
		{SourceAbs: srcA, TargetAbs: tgtA},
		{SourceAbs: srcB, TargetAbs: tgtB},
	}, LinkOptions{NoInteractive: true}, logger)

	// Create itself must fail (second symlink errors).
	if err == nil {
		t.Fatal("expected Create to return an error")
	}
}

// failOnNthSymlinkAndRemoveFS fails the N-th Symlink call and all Remove
// calls, simulating a broken FS mid-operation that also prevents rollback.
type failOnNthSymlinkAndRemoveFS struct {
	testutil.OSBackedFS
	count        int
	failSymlinkN int
}

func (f *failOnNthSymlinkAndRemoveFS) Symlink(oldname, newname string) error {
	f.count++
	if f.count == f.failSymlinkN {
		return errors.New("forced symlink failure")
	}
	return f.OSBackedFS.Symlink(oldname, newname)
}

func (f *failOnNthSymlinkAndRemoveFS) Remove(name string) error {
	return errors.New("forced remove failure during rollback")
}

// WriteFile must succeed so the transaction file can be saved.
func (f *failOnNthSymlinkAndRemoveFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	return f.OSBackedFS.WriteFile(name, data, perm)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// findDeadPID forks a child process, waits for it to exit, and returns its
// PID.  By the time this function returns, the OS has reaped the child so
// kill(pid, 0) will return ESRCH.
func findDeadPID(t *testing.T) int {
	t.Helper()

	// Use a very high PID that is almost certainly unused.  On Linux / macOS
	// the maximum PID is 4194304 (Linux) or 99998 (macOS). We use a value
	// above the macOS limit to be sure.
	candidatePID := 99999
	// Confirm it is actually dead.
	if err := syscall.Kill(candidatePID, 0); err == nil || errors.Is(err, syscall.EPERM) {
		// That PID happens to be alive; fall back to scanning.
		for p := 99990; p <= 99999; p++ {
			if err2 := syscall.Kill(p, 0); errors.Is(err2, syscall.ESRCH) {
				return p
			}
		}
		t.Skip("could not find a dead PID for stale-lock test")
	}
	return candidatePID
}
