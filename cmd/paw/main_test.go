package main

import (
	"bytes"
	"errors"
	"os"
	"testing"
)

func TestRunUsesExecuteRoot(t *testing.T) {
	orig := executeRoot
	t.Cleanup(func() { executeRoot = orig })

	executeRoot = func() error { return nil }
	if err := run(); err != nil {
		t.Fatal(err)
	}

	want := errors.New("boom")
	executeRoot = func() error { return want }
	if err := run(); !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}
}

func TestMainHandlesError(t *testing.T) {
	origExecute := executeRoot
	origExit := exitFn
	origErrOut := errOut
	t.Cleanup(func() {
		executeRoot = origExecute
		exitFn = origExit
		errOut = origErrOut
	})

	var code int
	buf := &bytes.Buffer{}
	executeRoot = func() error { return errors.New("failure") }
	exitFn = func(c int) { code = c }
	errOut = buf

	main()

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if buf.Len() == 0 {
		t.Fatal("expected error output")
	}
}

func TestExecuteRootDefaultImplementation(t *testing.T) {
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })

	os.Args = []string{"paw", "--help"}
	if err := executeRoot(); err != nil {
		t.Fatal(err)
	}
}
