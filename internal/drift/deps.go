package drift

import (
	"github.com/alexcatdad/paw/internal/clock"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/fsx"
)

var (
	runner execx.Runner = execx.NewOSRunner()
	fsys   fsx.FS       = fsx.NewOSFS()
	clk    clock.Clock  = clock.RealClock{}
)

func SetDependencies(r execx.Runner, f fsx.FS, c clock.Clock) {
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
