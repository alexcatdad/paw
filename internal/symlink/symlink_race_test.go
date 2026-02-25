package symlink

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/alexcatdad/paw/internal/clock"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/fsx"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/testutil"
)

// TestSetDependenciesConcurrentRace verifies that concurrent calls to
// SetDependencies and symlink operations (which read globals via accessors)
// do not trigger a data race under -race.
//
// Note: t.Parallel() cannot be combined with t.Setenv in Go 1.26+,
// so concurrency is exercised via goroutines within this single test.
func TestSetDependenciesConcurrentRace(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repoDir := t.TempDir()

	// Create a source file to link.
	source := filepath.Join(repoDir, "home", ".zshrc-race")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("# race test"), 0o644); err != nil {
		t.Fatal(err)
	}

	logger := output.NewLogger("text", true, false)

	const goroutines = 8
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Half the goroutines call SetDependencies concurrently (writes under mu.Lock).
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			SetDependencies(
				&testutil.FakeRunner{},
				fsx.NewOSFS(),
				clock.RealClock{},
			)
		}()
	}

	// Other half call Status (reads globals via getFsys() under mu.RLock).
	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			target := filepath.Join(home, fmt.Sprintf(".zshrc-race-%d", n))
			// Status reads fsys.Stat via getFsys() — must not race with SetDependencies.
			_, _ = Status([]Entry{{
				SourceRel: "home/.zshrc-race",
				SourceAbs: source,
				TargetRel: fmt.Sprintf(".zshrc-race-%d", n),
				TargetAbs: target,
			}})
		}(i)
	}

	wg.Wait()

	// Second round: concurrent Create + SetDependencies.
	wg.Add(goroutines * 2)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			SetDependencies(
				&testutil.FakeRunner{},
				fsx.NewOSFS(),
				clock.RealClock{},
			)
		}()
	}
	for i := 0; i < goroutines; i++ {
		targetDir := t.TempDir()
		go func(dir string) {
			defer wg.Done()
			target := filepath.Join(dir, ".zshrc-race")
			// Create reads getRunner(), getFsys(), getClk() — must not race with SetDependencies.
			_, _ = Create([]Entry{{
				SourceRel: "home/.zshrc-race",
				SourceAbs: source,
				TargetRel: ".zshrc-race",
				TargetAbs: target,
			}}, LinkOptions{DryRun: true, NoInteractive: true}, logger)
		}(targetDir)
	}
	wg.Wait()

	// Restore defaults.
	SetDependencies(execx.NewOSRunner(), fsx.NewOSFS(), clock.RealClock{})
}
