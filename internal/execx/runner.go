package execx

import (
	"context"
	"io"
	"os/exec"
)

type CommandOptions struct {
	Env    []string
	Dir    string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type Runner interface {
	LookPath(file string) (string, error)
	Run(name string, args ...string) error
	RunContext(ctx context.Context, name string, args ...string) error
	Output(name string, args ...string) ([]byte, error)
	OutputContext(ctx context.Context, name string, args ...string) ([]byte, error)
	CombinedOutput(name string, args ...string) ([]byte, error)
	CombinedOutputContext(ctx context.Context, name string, args ...string) ([]byte, error)
	RunWith(name string, args []string, opts CommandOptions) error
	RunWithContext(ctx context.Context, name string, args []string, opts CommandOptions) error
}

type OSRunner struct{}

func NewOSRunner() Runner {
	return OSRunner{}
}

func (OSRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (OSRunner) Run(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

func (OSRunner) RunContext(ctx context.Context, name string, args ...string) error {
	return exec.CommandContext(ctx, name, args...).Run()
}

func (OSRunner) Output(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

func (OSRunner) OutputContext(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

func (OSRunner) CombinedOutput(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

func (OSRunner) CombinedOutputContext(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func (OSRunner) RunWith(name string, args []string, opts CommandOptions) error {
	return runWithCmd(exec.Command(name, args...), opts)
}

func (OSRunner) RunWithContext(ctx context.Context, name string, args []string, opts CommandOptions) error {
	return runWithCmd(exec.CommandContext(ctx, name, args...), opts)
}

func runWithCmd(cmd *exec.Cmd, opts CommandOptions) error {
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}
	if len(opts.Env) > 0 {
		cmd.Env = opts.Env
	}
	if opts.Stdin != nil {
		cmd.Stdin = opts.Stdin
	}
	if opts.Stdout != nil {
		cmd.Stdout = opts.Stdout
	}
	if opts.Stderr != nil {
		cmd.Stderr = opts.Stderr
	}
	return cmd.Run()
}
