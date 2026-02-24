package fsx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOSFSOperations(t *testing.T) {
	base := t.TempDir()
	fs := NewOSFS()
	dir := filepath.Join(base, "a", "b")
	if err := fs.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "file.txt")
	if err := fs.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(file); err != nil {
		t.Fatal(err)
	}
	data, err := fs.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected file content: %q", string(data))
	}
	link := filepath.Join(base, "link.txt")
	if err := fs.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Lstat(link); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Readlink(link); err != nil {
		t.Fatal(err)
	}
	if err := fs.Chmod(file, 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := fs.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	renamed := filepath.Join(dir, "renamed.txt")
	if err := fs.Rename(file, renamed); err != nil {
		t.Fatal(err)
	}
	f, err := fs.OpenFile(renamed, os.O_RDONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if err := fs.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := fs.RemoveAll(base); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(base); !os.IsNotExist(err) {
		t.Fatalf("expected base removed, err=%v", err)
	}
}
