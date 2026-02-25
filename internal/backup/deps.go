package backup

import (
	"sync"

	"github.com/alexcatdad/paw/internal/clock"
	"github.com/alexcatdad/paw/internal/fsx"
)

var (
	mu   sync.RWMutex
	fsys fsx.FS      = fsx.NewOSFS()
	clk  clock.Clock = clock.RealClock{}
)

func SetDependencies(f fsx.FS, c clock.Clock) {
	mu.Lock()
	defer mu.Unlock()
	if f != nil {
		fsys = f
	}
	if c != nil {
		clk = c
	}
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
