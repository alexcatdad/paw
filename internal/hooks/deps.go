package hooks

import (
	"sync"

	"github.com/alexcatdad/paw/internal/execx"
)

var (
	mu     sync.RWMutex
	runner execx.Runner = execx.NewOSRunner()
)

func SetDependencies(r execx.Runner) {
	mu.Lock()
	defer mu.Unlock()
	if r != nil {
		runner = r
	}
}

func getRunner() execx.Runner {
	mu.RLock()
	defer mu.RUnlock()
	return runner
}
