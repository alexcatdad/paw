package repo

import (
	"sync"

	"github.com/alexcatdad/paw/internal/clock"
	"github.com/alexcatdad/paw/internal/execx"
)

var (
	mu     sync.RWMutex
	runner execx.Runner = execx.NewOSRunner()
	clk    clock.Clock  = clock.RealClock{}
)

func SetDependencies(r execx.Runner, c clock.Clock) {
	mu.Lock()
	defer mu.Unlock()
	if r != nil {
		runner = r
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

func getClk() clock.Clock {
	mu.RLock()
	defer mu.RUnlock()
	return clk
}
