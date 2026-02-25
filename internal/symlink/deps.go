package symlink

import (
	"sync"

	"github.com/alexcatdad/paw/internal/clock"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/fsx"
)

var (
	mu     sync.RWMutex
	runner execx.Runner = execx.NewOSRunner()
	fsys   fsx.FS       = fsx.NewOSFS()
	clk    clock.Clock  = clock.RealClock{}
)

func SetDependencies(r execx.Runner, f fsx.FS, c clock.Clock) {
	mu.Lock()
	defer mu.Unlock()
	if r != nil {
		runner = r
	}
	if f != nil {
		fsys = f
	}
	if c != nil {
		clk = c
	}
}

func getRunner() execx.Runner {
	mu.RLock()
	defer mu.RUnlock()
	return runner
}

func getFsys() fsx.FS {
	mu.RLock()
	defer mu.RUnlock()
	return fsys
}

func getClk() clock.Clock {
	mu.RLock()
	defer mu.RUnlock()
	return clk
}
