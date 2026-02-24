package packages

import "github.com/alexcatdad/paw/internal/execx"

var runner execx.Runner = execx.NewOSRunner()

func SetDependencies(r execx.Runner) {
	if r != nil {
		runner = r
	}
}
