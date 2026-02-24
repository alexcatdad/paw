package state

import (
	"os"
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
	if _, err := LockPath(); err != nil {
		t.Fatal(err)
	}
	if _, err := LegacyLastRunPath(); err != nil {
		t.Fatal(err)
	}
}
