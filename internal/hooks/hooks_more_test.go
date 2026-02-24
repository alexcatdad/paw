package hooks

import (
	"errors"
	"strings"
	"testing"

	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/testutil"
)

func withRunner(t *testing.T, r execx.Runner) {
	t.Helper()
	SetDependencies(r)
	t.Cleanup(func() { SetDependencies(execx.NewOSRunner()) })
}

func TestRunSkipAndNoHook(t *testing.T) {
	cfg := config.Default()
	logger := output.NewLogger("text", true, false)
	if err := Run("pre_install", cfg, Options{SkipHooks: true}, logger); err != nil {
		t.Fatal(err)
	}
	if err := Run("pre_install", cfg, Options{}, logger); err != nil {
		t.Fatal(err)
	}
}

func TestRunHookFailure(t *testing.T) {
	cfg := config.Default()
	cfg.Hooks.PreInstall = "echo test"
	fake := &testutil.FakeRunner{
		RunWithFn: func(string, []string, execx.CommandOptions) error {
			return errors.New("boom")
		},
	}
	withRunner(t, fake)
	logger := output.NewLogger("text", true, false)
	err := Run("pre_install", cfg, Options{}, logger)
	if err == nil || !strings.Contains(err.Error(), "hook pre_install failed") {
		t.Fatalf("expected hook failure, got: %v", err)
	}
}

func TestRunHookSuccessAndSelectHookSwitch(t *testing.T) {
	cfg := config.Default()
	cfg.Hooks = config.HookConfig{
		PreInstall:   "echo pre_install",
		PostInstall:  "echo post_install",
		PreLink:      "echo pre_link",
		PostLink:     "echo post_link",
		PreSync:      "echo pre_sync",
		PostSync:     "echo post_sync",
		PrePush:      "echo pre_push",
		PostPush:     "echo post_push",
		PreUpdate:    "echo pre_update",
		PostUpdate:   "echo post_update",
		PreRollback:  "echo pre_rollback",
		PostRollback: "echo post_rollback",
	}
	fake := &testutil.FakeRunner{}
	withRunner(t, fake)
	logger := output.NewLogger("text", true, false)

	names := []string{
		"pre_install", "post_install", "pre_link", "post_link",
		"pre_sync", "post_sync", "pre_push", "post_push",
		"pre_update", "post_update", "pre_rollback", "post_rollback",
	}
	for _, name := range names {
		if err := Run(name, cfg, Options{}, logger); err != nil {
			t.Fatalf("run %s failed: %v", name, err)
		}
	}
	if got := selectHook("unknown", cfg.Hooks); got != "" {
		t.Fatalf("expected empty unknown hook, got %q", got)
	}
	if len(fake.Calls) < len(names) {
		t.Fatalf("expected runner calls for hooks, got %d", len(fake.Calls))
	}
}
