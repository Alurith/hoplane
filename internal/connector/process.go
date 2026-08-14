package connector

import (
	"context"
	"os/exec"
)

// ProcessRunner abstracts process lookup and execution for connector tests.
type ProcessRunner interface {
	LookPath(string) (string, error)
	Run(context.Context, Invocation, IO) error
}

// ExecRunner runs processes using the host operating system.
type ExecRunner struct{}

func (ExecRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func (ExecRunner) Run(ctx context.Context, invocation Invocation, streams IO) error {
	command := exec.CommandContext(ctx, invocation.Program, invocation.Args...)
	command.Stdin = streams.Input
	command.Stdout = streams.Output
	command.Stderr = streams.Errors
	return command.Run()
}
