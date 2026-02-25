package symlink

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/platform"
	"github.com/alexcatdad/paw/internal/repo"
	"github.com/alexcatdad/paw/internal/state"
)

const (
	StatusLinked        = "linked"
	StatusMissing       = "missing"
	StatusConflict      = "conflict"
	StatusSourceMissing = "source-missing"
	StatusBackup        = "backup"
	StatusSkipped       = "skipped"
)

type Entry struct {
	SourceRel string
	SourceAbs string
	TargetRel string
	TargetAbs string
	Platform  []string
	Hostname  string
}

type State struct {
	Source     string `json:"source"`
	Target     string `json:"target"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
	BackupPath string `json:"backupPath,omitempty"`
}

type LinkOptions struct {
	DryRun        bool
	Force         bool
	NoInteractive bool
}

type conflictChoice struct {
	Action     string
	ApplyToAll bool
}

type transaction struct {
	Created []string     `json:"created"`
	Backups []backupMove `json:"backups"`
}

type backupMove struct {
	Original string `json:"original"`
	Backup   string `json:"backup"`
}

func BuildEntries(repoDir string, homeDir string, cfg config.Config) ([]Entry, error) {
	entries := map[string]Entry{}
	homeRoot := filepath.Join(repoDir, "home")
	// Fix M2: propagate walk errors instead of silently dropping them.
	if err := filepath.WalkDir(homeRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(homeRoot, path)
		if err != nil {
			return err
		}
		targetRel := filepath.ToSlash(rel)
		if isIgnored(targetRel, cfg.Ignore.Paths) {
			return nil
		}
		sourceRel := filepath.ToSlash(filepath.Join("home", rel))
		entries[sourceRel] = Entry{
			SourceRel: sourceRel,
			SourceAbs: path,
			TargetRel: targetRel,
			TargetAbs: filepath.Join(homeDir, rel),
		}
		return nil
	}); err != nil {
		// homeRoot may not exist yet (empty or newly-initialized repo).
		// Treat ErrNotExist as non-fatal; propagate all other errors.
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("walk %s: %w", homeRoot, err)
		}
	}

	for sourceRel, override := range cfg.Overrides {
		sourcePath := filepath.Join(repoDir, filepath.FromSlash(sourceRel))
		targetPath, err := repo.ExpandHome(override.Target)
		if err != nil {
			return nil, err
		}
		if !filepath.IsAbs(targetPath) {
			targetPath = filepath.Join(homeDir, filepath.FromSlash(targetPath))
		}
		targetRel, err := filepath.Rel(homeDir, targetPath)
		if err != nil {
			return nil, err
		}
		entries[sourceRel] = Entry{
			SourceRel: sourceRel,
			SourceAbs: sourcePath,
			TargetRel: filepath.ToSlash(targetRel),
			TargetAbs: targetPath,
			Platform:  override.Platform,
			Hostname:  override.Hostname,
		}
	}

	result := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].TargetAbs < result[j].TargetAbs
	})
	return result, nil
}

func Create(entries []Entry, opts LinkOptions, logger *output.Logger) ([]State, error) {
	unlock, err := acquireLock()
	if err != nil {
		return nil, err
	}
	defer unlock()

	txn := transaction{Created: []string{}, Backups: []backupMove{}}
	defer cleanupTransactionFile()

	states := make([]State, 0, len(entries))
	var pending *conflictChoice
	currentPlatform := platform.Current()
	hostname := platform.Hostname()

	for _, entry := range entries {
		if err := validateTarget(entry.TargetAbs); err != nil {
			return states, err
		}
		if !platform.MatchPlatform(entry.Platform, currentPlatform) {
			states = append(states, State{Source: entry.SourceAbs, Target: entry.TargetAbs, Status: StatusSkipped, Reason: "platform mismatch"})
			logger.Info(fmt.Sprintf("Skipped %s (platform mismatch)", repo.ContractHome(entry.TargetAbs)))
			continue
		}
		if !platform.MatchHostname(entry.Hostname, hostname) {
			states = append(states, State{Source: entry.SourceAbs, Target: entry.TargetAbs, Status: StatusSkipped, Reason: "hostname mismatch"})
			logger.Info(fmt.Sprintf("Skipped %s (hostname mismatch)", repo.ContractHome(entry.TargetAbs)))
			continue
		}

		st, nextPending, applyErr := createOne(entry, opts, pending, &txn, logger)
		if st.Source != "" {
			states = append(states, st)
		}
		if applyErr != nil {
			if !opts.DryRun {
				// Fix M8 caller: handle the rollback error instead of discarding it.
				if rbErr := rollbackTransaction(txn, logger); rbErr != nil {
					logger.Warn(fmt.Sprintf("Rollback encountered errors: %v", rbErr))
				}
			}
			return states, applyErr
		}
		pending = nextPending
	}
	return states, nil
}

func createOne(entry Entry, opts LinkOptions, pending *conflictChoice, txn *transaction, logger *output.Logger) (State, *conflictChoice, error) {
	st := State{Source: entry.SourceAbs, Target: entry.TargetAbs, Status: StatusMissing}
	if _, err := getFsys().Stat(entry.SourceAbs); err != nil {
		logger.Error(fmt.Sprintf("Source not found: %s", repo.ContractHome(entry.SourceAbs)))
		st.Status = StatusSourceMissing
		return st, pending, nil
	}

	exists, err := fileOrLinkExists(entry.TargetAbs)
	if err != nil {
		return st, pending, err
	}

	if exists {
		correct, err := isSymlinkTo(entry.TargetAbs, entry.SourceAbs)
		if err != nil {
			return st, pending, err
		}
		if correct {
			st.Status = StatusLinked
			logger.Info(fmt.Sprintf("Already linked: %s", repo.ContractHome(entry.TargetAbs)))
			return st, pending, nil
		}

		choice := pending
		if choice == nil {
			if opts.Force {
				choice = &conflictChoice{Action: "backup"}
			} else if opts.NoInteractive || !isInteractive() {
				st.Status = StatusConflict
				logger.Warn(fmt.Sprintf("Conflict: %s exists (use --force)", repo.ContractHome(entry.TargetAbs)))
				return st, pending, nil
			} else {
				chosen, err := promptConflict(entry.TargetAbs, entry.SourceAbs)
				if err != nil {
					return st, pending, err
				}
				choice = &chosen
			}
		}

		switch choice.Action {
		case "abort":
			return st, pending, errors.New("aborted by user")
		case "skip":
			st.Status = StatusConflict
			logger.Info(fmt.Sprintf("Skipped by user: %s", repo.ContractHome(entry.TargetAbs)))
			if choice.ApplyToAll {
				return st, choice, nil
			}
			return st, nil, nil
		case "overwrite":
			if opts.DryRun {
				logger.DryRun(fmt.Sprintf("Would overwrite %s", repo.ContractHome(entry.TargetAbs)))
			} else {
				if err := getFsys().Remove(entry.TargetAbs); err != nil {
					return st, pending, err
				}
			}
		case "backup":
			backupPath := fmt.Sprintf("%s.backup.%d", entry.TargetAbs, getClk().Now().UnixMilli())
			if opts.DryRun {
				logger.DryRun(fmt.Sprintf("Would backup %s -> %s", repo.ContractHome(entry.TargetAbs), repo.ContractHome(backupPath)))
			} else {
				if err := getFsys().Rename(entry.TargetAbs, backupPath); err != nil {
					return st, pending, err
				}
				txn.Backups = append(txn.Backups, backupMove{Original: entry.TargetAbs, Backup: backupPath})
				if err := saveTransaction(*txn); err != nil {
					return st, pending, err
				}
			}
			st.BackupPath = backupPath
			st.Status = StatusBackup
		}

		if choice.ApplyToAll {
			pending = choice
		} else {
			pending = nil
		}
	}

	if opts.DryRun {
		logger.DryRun(fmt.Sprintf("Would symlink %s -> %s", repo.ContractHome(entry.TargetAbs), repo.ContractHome(entry.SourceAbs)))
		if st.Status != StatusBackup {
			st.Status = StatusLinked
		}
		return st, pending, nil
	}

	if err := getFsys().MkdirAll(filepath.Dir(entry.TargetAbs), 0o755); err != nil {
		return st, pending, err
	}
	if err := getFsys().Symlink(entry.SourceAbs, entry.TargetAbs); err != nil {
		return st, pending, err
	}
	txn.Created = append(txn.Created, entry.TargetAbs)
	if err := saveTransaction(*txn); err != nil {
		return st, pending, err
	}
	if st.Status != StatusBackup {
		st.Status = StatusLinked
	}
	logger.Success(fmt.Sprintf("Linked %s", repo.ContractHome(entry.TargetAbs)))
	return st, pending, nil
}

func Remove(entries []Entry, opts LinkOptions, logger *output.Logger) error {
	for _, entry := range entries {
		if err := validateTarget(entry.TargetAbs); err != nil {
			return err
		}
		info, err := getFsys().Lstat(entry.TargetAbs)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			logger.Warn(fmt.Sprintf("Skipping non-symlink: %s", repo.ContractHome(entry.TargetAbs)))
			continue
		}
		if opts.DryRun {
			logger.DryRun(fmt.Sprintf("Would remove symlink: %s", repo.ContractHome(entry.TargetAbs)))
			continue
		}
		if err := getFsys().Remove(entry.TargetAbs); err != nil {
			return err
		}
		logger.Success(fmt.Sprintf("Removed %s", repo.ContractHome(entry.TargetAbs)))
	}
	return nil
}

func Status(entries []Entry) ([]State, error) {
	states := make([]State, 0, len(entries))
	currentPlatform := platform.Current()
	hostname := platform.Hostname()
	for _, entry := range entries {
		st := State{Source: entry.SourceAbs, Target: entry.TargetAbs, Status: StatusMissing}
		if !platform.MatchPlatform(entry.Platform, currentPlatform) {
			st.Status = StatusSkipped
			st.Reason = "platform mismatch"
			states = append(states, st)
			continue
		}
		if !platform.MatchHostname(entry.Hostname, hostname) {
			st.Status = StatusSkipped
			st.Reason = "hostname mismatch"
			states = append(states, st)
			continue
		}
		if _, err := getFsys().Stat(entry.SourceAbs); err != nil {
			st.Status = StatusSourceMissing
			states = append(states, st)
			continue
		}
		exists, err := fileOrLinkExists(entry.TargetAbs)
		if err != nil {
			return nil, err
		}
		if !exists {
			st.Status = StatusMissing
			states = append(states, st)
			continue
		}
		isLinked, err := isSymlinkTo(entry.TargetAbs, entry.SourceAbs)
		if err != nil {
			return nil, err
		}
		if isLinked {
			st.Status = StatusLinked
		} else {
			st.Status = StatusConflict
		}
		states = append(states, st)
	}
	return states, nil
}

func isIgnored(targetRel string, ignore []string) bool {
	for _, path := range ignore {
		clean := strings.TrimSpace(strings.TrimPrefix(filepath.ToSlash(path), "~/"))
		if clean == "" {
			continue
		}
		if targetRel == clean || targetRel == strings.TrimPrefix(clean, ".") {
			return true
		}
	}
	return false
}

func fileOrLinkExists(path string) (bool, error) {
	_, err := getFsys().Lstat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func isSymlinkTo(linkPath string, expectedTarget string) (bool, error) {
	info, err := getFsys().Lstat(linkPath)
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return false, nil
	}
	actual, err := getFsys().Readlink(linkPath)
	if err != nil {
		return false, err
	}
	resolvedActual := actual
	if !filepath.IsAbs(actual) {
		resolvedActual = filepath.Join(filepath.Dir(linkPath), actual)
	}
	resolvedActual = filepath.Clean(resolvedActual)
	resolvedExpected := filepath.Clean(expectedTarget)
	return resolvedActual == resolvedExpected || filepath.Clean(actual) == resolvedExpected, nil
}

func validateTarget(target string) error {
	home, err := repo.HomeDir()
	if err != nil {
		return err
	}
	return repo.ValidateWithinBase(target, home)
}

func isInteractive() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func promptConflict(target string, source string) (conflictChoice, error) {
	fmt.Printf("\nConflict: %s already exists\n", repo.ContractHome(target))
	fmt.Println("  [s] Skip")
	fmt.Println("  [b] Backup & link")
	fmt.Println("  [o] Overwrite")
	fmt.Println("  [d] Show diff")
	fmt.Println("  [a] Abort")
	fmt.Println("  [S] Skip all")
	fmt.Println("  [B] Backup all")
	fmt.Print("Choice [s/b/o/d/a/S/B]: ")

	reader := bufio.NewReader(os.Stdin)
	for {
		char, _, err := reader.ReadRune()
		if err != nil {
			return conflictChoice{}, err
		}
		fmt.Println(string(char))
		switch char {
		case 's':
			return conflictChoice{Action: "skip"}, nil
		case 'b':
			return conflictChoice{Action: "backup"}, nil
		case 'o':
			return conflictChoice{Action: "overwrite"}, nil
		case 'a':
			return conflictChoice{Action: "abort"}, nil
		case 'S':
			return conflictChoice{Action: "skip", ApplyToAll: true}, nil
		case 'B':
			return conflictChoice{Action: "backup", ApplyToAll: true}, nil
		case 'd':
			showDiff(target, source)
			fmt.Print("Choice [s/b/o/d/a/S/B]: ")
		default:
			fmt.Print("Invalid choice [s/b/o/d/a/S/B]: ")
		}
	}
}

func showDiff(target string, source string) {
	_ = getRunner().RunWith("diff", []string{"-u", target, source}, execx.CommandOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
}

// pidIsAlive checks whether the process with the given PID is still running.
// Returns true if alive (kill(pid, 0) returned nil or EPERM),
// false if the process is definitively dead (ESRCH).
func pidIsAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	if errors.Is(err, syscall.EPERM) {
		// Process exists but we lack permission to signal it.
		return true
	}
	// syscall.ESRCH — no such process; stale lock.
	return false
}

// readPIDFromLockFile reads the PID stored in a lock file as "pid=<N>\n".
// Returns 0 and a non-nil error when the file cannot be read or parsed.
func readPIDFromLockFile(path string) (int, error) {
	data, err := getFsys().ReadFile(path)
	if err != nil {
		return 0, err
	}
	line := strings.TrimSpace(string(data))
	if !strings.HasPrefix(line, "pid=") {
		return 0, fmt.Errorf("unexpected lock file format: %q", line)
	}
	pid, err := strconv.Atoi(strings.TrimPrefix(line, "pid="))
	if err != nil {
		return 0, fmt.Errorf("invalid PID in lock file: %w", err)
	}
	return pid, nil
}

func acquireLock() (func(), error) {
	lockPath, err := state.LockPath()
	if err != nil {
		return nil, err
	}

	// Fix I4: use 0o600 instead of 0o644.
	f, err := getFsys().OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err == nil {
		_, _ = f.WriteString(fmt.Sprintf("pid=%d\n", os.Getpid()))
		_ = f.Close()
		return func() { _ = getFsys().Remove(lockPath) }, nil
	}

	// Fix I1: stale lock recovery.
	// The lock file already exists. Determine whether the owning process is
	// still alive before refusing to proceed.
	pid, parseErr := readPIDFromLockFile(lockPath)
	if parseErr != nil {
		// Cannot read/parse lock file — treat as a live lock to be safe.
		return nil, fmt.Errorf("another paw operation is running")
	}

	if pidIsAlive(pid) {
		return nil, fmt.Errorf("another paw operation is running (pid=%d)", pid)
	}

	// The process is gone — stale lock. Remove it and retry once.
	if removeErr := getFsys().Remove(lockPath); removeErr != nil {
		return nil, fmt.Errorf("another paw operation is running (pid=%d)", pid)
	}

	f, err = getFsys().OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("another paw operation is running")
	}
	_, _ = f.WriteString(fmt.Sprintf("pid=%d\n", os.Getpid()))
	_ = f.Close()
	return func() { _ = getFsys().Remove(lockPath) }, nil
}

func saveTransaction(txn transaction) error {
	path, err := state.TransactionPath()
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(txn, "", "  ")
	if err != nil {
		return err
	}
	return getFsys().WriteFile(path, payload, 0o644)
}

func cleanupTransactionFile() {
	path, err := state.TransactionPath()
	if err == nil {
		_ = getFsys().Remove(path)
	}
}

// rollbackTransaction reverses a transaction by removing created symlinks and
// restoring backed-up files. It collects all errors encountered during rollback
// and returns a combined error if any step failed. Fix M8: previously always
// returned nil.
func rollbackTransaction(txn transaction, logger *output.Logger) error {
	var errs []string

	for i := len(txn.Created) - 1; i >= 0; i-- {
		if err := getFsys().Remove(txn.Created[i]); err != nil {
			msg := fmt.Sprintf("failed to remove created link %s: %v", repo.ContractHome(txn.Created[i]), err)
			logger.Warn(msg)
			errs = append(errs, msg)
		}
	}
	for i := len(txn.Backups) - 1; i >= 0; i-- {
		move := txn.Backups[i]
		_ = getFsys().Remove(move.Original)
		if err := getFsys().Rename(move.Backup, move.Original); err != nil {
			msg := fmt.Sprintf("Failed to restore %s: %v", repo.ContractHome(move.Original), err)
			logger.Warn(msg)
			errs = append(errs, msg)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("rollback errors: %s", strings.Join(errs, "; "))
	}
	return nil
}
