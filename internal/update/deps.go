package update

import (
	"github.com/alexcatdad/paw/internal/clock"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/fsx"
)

var (
	runner execx.Runner = execx.NewOSRunner()
	clk    clock.Clock  = clock.RealClock{}
	fsys   fsx.FS       = fsx.NewOSFS()
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
