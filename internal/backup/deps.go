package backup

import (
	"github.com/alexcatdad/paw/internal/clock"
	"github.com/alexcatdad/paw/internal/fsx"
)

var (
	fsys fsx.FS      = fsx.NewOSFS()
	clk  clock.Clock = clock.RealClock{}
)

func SetDependencies(f fsx.FS, c clock.Clock) {
	if f != nil {
		fsys = f
	}
	if c != nil {
		clk = c
	}
}
