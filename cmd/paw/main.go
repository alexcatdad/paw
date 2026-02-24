package main

import (
	"fmt"
	"os"

	"github.com/alexcatdad/paw/internal/app"
	"github.com/alexcatdad/paw/internal/cli"
)

func main() {
	root := cli.NewRootCommand()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(app.ExitCode(err))
	}
}
