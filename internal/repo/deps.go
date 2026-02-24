package repo

import (
	"github.com/alexcatdad/paw/internal/clock"
	"github.com/alexcatdad/paw/internal/execx"
)

var (
	runner execx.Runner = execx.NewOSRunner()
	clk    clock.Clock  = clock.RealClock{}
)

func SetDependencies(r execx.Runner, c clock.Clock) {
	if r != nil {
		runner = r
	}
	if c != nil {
		clk = c
	}
}
