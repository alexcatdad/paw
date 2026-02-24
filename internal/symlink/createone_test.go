package symlink

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alexcatdad/paw/internal/output"
)

func TestCreateOneBranches(t *testing.T) {
	home := t.TempDir()
	repoDir := t.TempDir()
	t.Setenv("HOME", home)
	source := filepath.Join(repoDir, "home", ".zshrc")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, ".zshrc")
	logger := output.NewLogger("text", true, false)

	t.Run("source missing", func(t *testing.T) {
		st, _, err := createOne(Entry{
			SourceAbs: filepath.Join(repoDir, "home", ".missing"),
			TargetAbs: filepath.Join(home, ".missing"),
		}, LinkOptions{NoInteractive: true}, nil, &transaction{}, logger)
		if err != nil {
			t.Fatal(err)
		}
		if st.Status != StatusSourceMissing {
			t.Fatalf("expected source missing, got %s", st.Status)
		}
	})

	t.Run("already linked", func(t *testing.T) {
		if err := os.Symlink(source, target); err != nil {
			t.Fatal(err)
		}
		st, _, err := createOne(Entry{SourceAbs: source, TargetAbs: target}, LinkOptions{NoInteractive: true}, nil, &transaction{}, logger)
		if err != nil {
			t.Fatal(err)
		}
		if st.Status != StatusLinked {
			t.Fatalf("expected linked status, got %s", st.Status)
		}
		_ = os.Remove(target)
	})

	t.Run("overwrite choice", func(t *testing.T) {
		if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
			t.Fatal(err)
		}
		st, _, err := createOne(Entry{SourceAbs: source, TargetAbs: target}, LinkOptions{}, &conflictChoice{Action: "overwrite"}, &transaction{}, logger)
		if err != nil {
			t.Fatal(err)
		}
		if st.Status != StatusLinked {
			t.Fatalf("expected linked, got %s", st.Status)
		}
		_ = os.Remove(target)
	})

	t.Run("abort choice", func(t *testing.T) {
		if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, _, err := createOne(Entry{SourceAbs: source, TargetAbs: target}, LinkOptions{}, &conflictChoice{Action: "abort"}, &transaction{}, logger)
		if err == nil {
			t.Fatal("expected abort error")
		}
		_ = os.Remove(target)
	})

	t.Run("skip apply all", func(t *testing.T) {
		if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
			t.Fatal(err)
		}
		st, next, err := createOne(Entry{SourceAbs: source, TargetAbs: target}, LinkOptions{}, &conflictChoice{Action: "skip", ApplyToAll: true}, &transaction{}, logger)
		if err != nil {
			t.Fatal(err)
		}
		if st.Status != StatusConflict || next == nil || !next.ApplyToAll {
			t.Fatalf("unexpected skip result: state=%+v next=%+v", st, next)
		}
	})

	t.Run("backup dry run", func(t *testing.T) {
		if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
			t.Fatal(err)
		}
		st, _, err := createOne(Entry{SourceAbs: source, TargetAbs: target}, LinkOptions{DryRun: true}, &conflictChoice{Action: "backup"}, &transaction{}, logger)
		if err != nil {
			t.Fatal(err)
		}
		if st.Status != StatusBackup || st.BackupPath == "" {
			t.Fatalf("expected backup dry run state, got %+v", st)
		}
	})
}
