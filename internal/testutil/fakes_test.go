package testutil

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexcatdad/paw/internal/execx"
)

func TestFakeClockAndNopWriter(t *testing.T) {
	clk := FakeClock{}
	if clk.Now().IsZero() {
		t.Fatal("expected non-zero fallback clock value")
	}
	n, err := NopWriter{}.Write([]byte("abc"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("unexpected bytes written: %d", n)
	}
}

func TestFakeRunnerMethods(t *testing.T) {
	r := &FakeRunner{}
	if _, err := r.LookPath("git"); err != nil {
		t.Fatal(err)
	}
	if err := r.Run("git", "status"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Output("git", "rev-parse", "HEAD"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.CombinedOutput("git", "status", "--porcelain"); err != nil {
		t.Fatal(err)
	}
	if err := r.RunWith("sh", []string{"-c", "echo hi"}, execx.CommandOptions{}); err != nil {
		t.Fatal(err)
	}
	joined := r.JoinedCalls()
	for _, token := range []string{"git status", "git rev-parse HEAD", "git status --porcelain", "sh -c echo hi"} {
		if !strings.Contains(joined, token) {
			t.Fatalf("expected %q in calls: %s", token, joined)
		}
	}

	errRun := errors.New("run error")
	r = &FakeRunner{
		LookPathFn:       func(string) (string, error) { return "", errors.New("missing") },
		RunFn:            func(string, ...string) error { return errRun },
		OutputFn:         func(string, ...string) ([]byte, error) { return nil, errRun },
		CombinedOutputFn: func(string, ...string) ([]byte, error) { return nil, errRun },
		RunWithFn:        func(string, []string, execx.CommandOptions) error { return errRun },
	}
	if _, err := r.LookPath("git"); err == nil {
		t.Fatal("expected lookpath error")
	}
	if err := r.Run("git"); !errors.Is(err, errRun) {
		t.Fatalf("unexpected run error: %v", err)
	}
	if _, err := r.Output("git"); !errors.Is(err, errRun) {
		t.Fatalf("unexpected output error: %v", err)
	}
	if _, err := r.CombinedOutput("git"); !errors.Is(err, errRun) {
		t.Fatalf("unexpected combined error: %v", err)
	}
	if err := r.RunWith("git", nil, execx.CommandOptions{}); !errors.Is(err, errRun) {
		t.Fatalf("unexpected runwith error: %v", err)
	}
}

func TestOSBackedFS(t *testing.T) {
	base := t.TempDir()
	fs := OSBackedFS{}
	dir := filepath.Join(base, "dir")
	if err := fs.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "file")
	if err := fs.WriteFile(file, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := fs.ReadFile(file)
	if err != nil || string(data) != "abc" {
		t.Fatalf("unexpected read result: %q err=%v", string(data), err)
	}
	if _, err := fs.Stat(file); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Lstat(file); err != nil {
		t.Fatal(err)
	}
	entries, err := fs.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("unexpected readdir result: %d err=%v", len(entries), err)
	}
	link := filepath.Join(dir, "link")
	if err := fs.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Readlink(link); err != nil {
		t.Fatal(err)
	}
	if err := fs.Chmod(file, 0o644); err != nil {
		t.Fatal(err)
	}
	renamed := filepath.Join(dir, "renamed")
	if err := fs.Rename(file, renamed); err != nil {
		t.Fatal(err)
	}
	opened, err := fs.OpenFile(renamed, os.O_RDONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_ = opened.Close()
	if err := fs.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := fs.RemoveAll(base); err != nil {
		t.Fatal(err)
	}
}

func TestFailingFS(t *testing.T) {
	want := errors.New("boom")
	fs := FailingFS{Err: want}
	if _, err := fs.ReadFile("x"); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if err := fs.WriteFile("x", []byte("a"), 0o644); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if _, err := fs.ReadDir("x"); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if err := fs.MkdirAll("x", 0o755); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if _, err := fs.OpenFile("x", os.O_RDONLY, 0o644); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if _, err := fs.Stat("x"); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if _, err := fs.Lstat("x"); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if _, err := fs.Readlink("x"); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if err := fs.Symlink("a", "b"); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if err := fs.Remove("x"); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if err := fs.RemoveAll("x"); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if err := fs.Rename("a", "b"); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if err := fs.Chmod("x", 0o644); !errors.Is(err, want) {
		t.Fatal(err)
	}
}
