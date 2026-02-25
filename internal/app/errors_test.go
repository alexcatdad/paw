package app

import (
	"errors"
	"testing"
)

func TestWithCodeAndExitCode(t *testing.T) {
	err := WithCode(ExitConfig, errors.New("bad config"))
	if ExitCode(err) != ExitConfig {
		t.Fatalf("expected %d got %d", ExitConfig, ExitCode(err))
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

func TestExitCodeNilReturnsOK(t *testing.T) {
	if ExitCode(nil) != ExitOK {
		t.Fatalf("expected ExitOK for nil error, got %d", ExitCode(nil))
	}
}

func TestExitCodeUnwrappedFallsBackToSystem(t *testing.T) {
	// Plain errors without a code always fall back to ExitSystem now that
	// substring matching has been removed.
	cases := []string{
		"unknown command",
		"paw.toml missing",
		"conflict exists",
		"hook pre failed",
		"rollback failed",
		"other error",
		"some random message",
	}
	for _, msg := range cases {
		if got := ExitCode(errors.New(msg)); got != ExitSystem {
			t.Fatalf("message %q: expected ExitSystem (%d) got %d", msg, ExitSystem, got)
		}
	}
}

func TestExitCodeWithCodeAllCodes(t *testing.T) {
	cases := []struct {
		code int
		name string
	}{
		{ExitOK, "ExitOK"},
		{ExitUsage, "ExitUsage"},
		{ExitConfig, "ExitConfig"},
		{ExitSystem, "ExitSystem"},
		{ExitConflict, "ExitConflict"},
		{ExitHookFailure, "ExitHookFailure"},
		{ExitRollbackError, "ExitRollbackError"},
	}
	for _, tc := range cases {
		err := WithCode(tc.code, errors.New("msg"))
		if got := ExitCode(err); got != tc.code {
			t.Fatalf("%s: WithCode round-trip failed: expected %d got %d", tc.name, tc.code, got)
		}
	}
}

func TestExitCodeWrappedPreservesCode(t *testing.T) {
	// Wrapping a codedError in another codedError: outermost code wins.
	inner := WithCode(ExitConfig, errors.New("config problem"))
	outer := WithCode(ExitHookFailure, inner)
	if got := ExitCode(outer); got != ExitHookFailure {
		t.Fatalf("expected ExitHookFailure (%d) from outer wrapper, got %d", ExitHookFailure, got)
	}
	// The inner code is accessible via errors.As on the inner value.
	if got := ExitCode(inner); got != ExitConfig {
		t.Fatalf("expected ExitConfig (%d) from inner, got %d", ExitConfig, got)
	}
}
