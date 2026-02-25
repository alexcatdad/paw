package hooks

import (
	"sync"
	"testing"

	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/testutil"
)

// TestSetDependenciesConcurrentRace verifies that concurrent calls to
// SetDependencies and Run do not trigger a data race under -race.
func TestSetDependenciesConcurrentRace(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Hooks.PreInstall = "echo concurrent"
	logger := output.NewLogger("text", true, false)

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Half the goroutines set dependencies concurrently.
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			fake := &testutil.FakeRunner{
				RunWithFn: func(string, []string, execx.CommandOptions) error {
					return nil
				},
			}
			SetDependencies(fake)
		}()
	}

	// Other half call Run concurrently (reads the global via getRunner()).
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			// Run reads getRunner() under mu.RLock — must not race with SetDependencies.
			_ = Run("pre_install", cfg, Options{}, logger)
		}()
	}

	wg.Wait()

	// Restore clean state.
	SetDependencies(execx.NewOSRunner())
}
