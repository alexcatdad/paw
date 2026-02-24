package main

import (
	"fmt"
	"io"
	"os"

	"github.com/alexcatdad/paw/internal/app"
	"github.com/alexcatdad/paw/internal/cli"
)

var executeRoot = func() error {
	root := cli.NewRootCommand()
	return root.Execute()
}

var exitFn = os.Exit
var errOut io.Writer = os.Stderr

func run() error {
	return executeRoot()
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(errOut, err)
		exitFn(app.ExitCode(err))
	}
}
