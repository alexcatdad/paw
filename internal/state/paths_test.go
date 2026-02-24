package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir, err := Ensure()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := LastRunPath(); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateStatePath(); err != nil {
		t.Fatal(err)
	}
	if _, err := TransactionPath(); err != nil {
		t.Fatal(err)
	}
	if _, err := LockPath(); err != nil {
		t.Fatal(err)
	}
	if _, err := LegacyLastRunPath(); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureAndDerivedPathsErrorWhenHomeIsFile(t *testing.T) {
	root := t.TempDir()
	homeFile := filepath.Join(root, "home-file")
	if err := os.WriteFile(homeFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeFile)

	if _, err := Ensure(); err == nil {
		t.Fatal("expected Ensure error")
	}
	if _, err := LastRunPath(); err == nil {
		t.Fatal("expected LastRunPath error")
	}
	if _, err := UpdateStatePath(); err == nil {
		t.Fatal("expected UpdateStatePath error")
	}
	if _, err := TransactionPath(); err == nil {
		t.Fatal("expected TransactionPath error")
	}
	if _, err := LockPath(); err == nil {
		t.Fatal("expected LockPath error")
	}
}
