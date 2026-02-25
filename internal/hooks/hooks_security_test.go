package hooks

import (
	"strings"
	"testing"

	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/testutil"
)

// ---------------------------------------------------------------------------
// Fix C1: hook confirmation
// ---------------------------------------------------------------------------

// TestRunConfirmYes verifies that when Confirm is enabled and the user types
// "y", the hook is executed.
func TestRunConfirmYes(t *testing.T) {
	cfg := config.Default()
	cfg.Hooks.PreInstall = "echo confirmed-hook"

	fakeRunner := &testutil.FakeRunner{}
	withRunner(t, fakeRunner)

	logger := output.NewLogger("text", true, false)
	err := Run("pre_install", cfg, Options{
		Confirm: true,
		Stdin:   strings.NewReader("y\n"),
	}, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fakeRunner.Calls) == 0 {
		t.Fatal("expected hook to be executed when user answers y")
	}
}

// TestRunConfirmNo verifies that when Confirm is enabled and the user types
// "n", the hook is skipped without error.
func TestRunConfirmNo(t *testing.T) {
	cfg := config.Default()
	cfg.Hooks.PreInstall = "echo should-not-run"

	fakeRunner := &testutil.FakeRunner{}
	withRunner(t, fakeRunner)

	logger := output.NewLogger("text", true, false)
	err := Run("pre_install", cfg, Options{
		Confirm: true,
		Stdin:   strings.NewReader("n\n"),
	}, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fakeRunner.Calls) != 0 {
		t.Fatalf("expected hook to be skipped when user answers n, got %d call(s)", len(fakeRunner.Calls))
	}
}

// TestRunConfirmEmptyAnswerSkips verifies that pressing Enter (empty answer)
// is treated as "N" (skip), consistent with the [y/N] default.
func TestRunConfirmEmptyAnswerSkips(t *testing.T) {
	cfg := config.Default()
	cfg.Hooks.PreInstall = "echo should-not-run"

	fakeRunner := &testutil.FakeRunner{}
	withRunner(t, fakeRunner)

	logger := output.NewLogger("text", true, false)
	err := Run("pre_install", cfg, Options{
		Confirm: true,
		Stdin:   strings.NewReader("\n"),
	}, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fakeRunner.Calls) != 0 {
		t.Fatalf("expected hook to be skipped on empty answer, got %d call(s)", len(fakeRunner.Calls))
	}
}

// TestRunConfirmCapitalY verifies that "Y" (uppercase) is also accepted.
func TestRunConfirmCapitalY(t *testing.T) {
	cfg := config.Default()
	cfg.Hooks.PreInstall = "echo capital-y"

	fakeRunner := &testutil.FakeRunner{}
	withRunner(t, fakeRunner)

	logger := output.NewLogger("text", true, false)
	err := Run("pre_install", cfg, Options{
		Confirm: true,
		Stdin:   strings.NewReader("Y\n"),
	}, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fakeRunner.Calls) == 0 {
		t.Fatal("expected hook to be executed when user answers Y")
	}
}

// TestRunNoConfirmSkipsPrompt verifies that when Confirm is false the hook
// runs without any stdin interaction (Stdin field is nil; a read would panic).
func TestRunNoConfirmSkipsPrompt(t *testing.T) {
	cfg := config.Default()
	cfg.Hooks.PreInstall = "echo no-confirm"

	fakeRunner := &testutil.FakeRunner{}
	withRunner(t, fakeRunner)

	logger := output.NewLogger("text", true, false)
	err := Run("pre_install", cfg, Options{Confirm: false}, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fakeRunner.Calls) == 0 {
		t.Fatal("expected hook to be executed without confirmation")
	}
}

// ---------------------------------------------------------------------------
// Fix M12: environment variable deduplication
// ---------------------------------------------------------------------------

// TestFilterEnvRemovesPrefix verifies that filterEnv strips all entries
// whose key starts with the given prefix.
func TestFilterEnvRemovesPrefix(t *testing.T) {
	input := []string{
		"HOME=/home/user",
		"PAW_HOME=/old/home",
		"PAW_REPO=/old/repo",
		"PAW_PLATFORM=linux",
		"PATH=/usr/bin",
		"PAWN=notmatching",
	}
	got := filterEnv(input, "PAW_")
	for _, e := range got {
		if strings.HasPrefix(e, "PAW_") {
			t.Errorf("filterEnv left PAW_ entry in result: %s", e)
		}
	}
	// Non-PAW_ entries (HOME, PATH, PAWN) must be preserved.
	if len(got) != 3 {
		t.Errorf("expected 3 entries after filtering, got %d: %v", len(got), got)
	}
}

// TestFilterEnvPreservesNonPrefix verifies that filterEnv is not too greedy:
// "PAWN" and "PAW" are not stripped when the prefix is "PAW_".
func TestFilterEnvPreservesNonPrefix(t *testing.T) {
	input := []string{
		"PAWN=chess",
		"PAW=bare",
		"PAW_SOMETHING=filtered",
	}
	got := filterEnv(input, "PAW_")
	if len(got) != 2 {
		t.Errorf("expected 2 entries (PAWN, PAW), got %d: %v", len(got), got)
	}
}

// TestRunEnvDedupNoDuplicates verifies that PAW_* variables appear exactly
// once in the env passed to the runner even when they already exist in the
// process environment. We inject a fake runner that captures
// execx.CommandOptions so we can inspect the Env slice.
func TestRunEnvDedupNoDuplicates(t *testing.T) {
	cfg := config.Default()
	cfg.Hooks.PreInstall = "echo dedup-test"

	var capturedEnv []string
	fake := &testutil.FakeRunner{
		RunWithFn: func(_ string, _ []string, opts execx.CommandOptions) error {
			capturedEnv = opts.Env
			return nil
		},
	}
	withRunner(t, fake)

	logger := output.NewLogger("text", true, false)
	if err := Run("pre_install", cfg, Options{}, logger); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Count occurrences of each PAW_ key in the captured env.
	counts := map[string]int{}
	for _, e := range capturedEnv {
		key := strings.SplitN(e, "=", 2)[0]
		if strings.HasPrefix(key, "PAW_") {
			counts[key]++
		}
	}
	for key, n := range counts {
		if n > 1 {
			t.Errorf("PAW_ var %s appears %d times in env (want exactly 1)", key, n)
		}
	}
	// The three PAW_ vars we inject must be present.
	for _, required := range []string{"PAW_PLATFORM", "PAW_HOME", "PAW_REPO"} {
		if counts[required] == 0 {
			t.Errorf("expected %s in env, not found", required)
		}
	}
}
