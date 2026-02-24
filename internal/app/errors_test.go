package app

import (
	"errors"
	"testing"
)

func TestWithCodeAndExitCode(t *testing.T) {
	err := WithCode(ExitConfig, errors.New("bad config"))
	if ExitCode(err) != ExitConfig {
		t.Fatalf("expected %d", ExitConfig)
	}
}

func TestCodedErrorMethods(t *testing.T) {
	root := errors.New("root")
	err := WithCode(ExitSystem, root)
	if err.Error() != "root" {
		t.Fatalf("unexpected error text: %s", err.Error())
	}
	var ce codedError
	if !errors.As(err, &ce) {
		t.Fatal("expected codedError type")
	}
	if !errors.Is(err, root) {
		t.Fatal("expected unwrapped root error")
	}
	if WithCode(ExitSystem, nil) != nil {
		t.Fatal("expected nil when wrapping nil error")
	}
}

func TestExitCodeHeuristics(t *testing.T) {
	cases := map[string]int{
		"unknown command":  ExitUsage,
		"paw.toml missing": ExitConfig,
		"conflict exists":  ExitConflict,
		"hook pre failed":  ExitHookFailure,
		"rollback failed":  ExitRollbackError,
		"other":            ExitSystem,
	}
	for msg, code := range cases {
		if got := ExitCode(errors.New(msg)); got != code {
			t.Fatalf("message %q expected %d got %d", msg, code, got)
		}
	}
}
