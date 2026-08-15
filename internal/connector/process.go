package connector

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

// ProcessRunner abstracts process lookup and execution for connector tests.
type ProcessRunner interface {
	LookPath(string) (string, error)
	Run(context.Context, Invocation, IO) error
}

// ExecRunner runs processes using the host operating system.
type ExecRunner struct{}

func (ExecRunner) LookPath(name string) (string, error) {
	if name != "ssh" && name != "xfreerdp" {
		return "", fmt.Errorf("executable %q is not allowlisted", name)
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve executable %q: %w", name, err)
	}
	if err := validateExecutable(path); err != nil {
		return "", fmt.Errorf("validate executable %q: %w", path, err)
	}
	return path, nil
}

func (ExecRunner) Run(ctx context.Context, invocation Invocation, streams IO) error {
	if !filepath.IsAbs(invocation.Program) {
		return fmt.Errorf("executable path must be absolute")
	}
	if err := validateExecutable(invocation.Program); err != nil {
		return fmt.Errorf("validate executable %q: %w", invocation.Program, err)
	}
	command := exec.CommandContext(ctx, invocation.Program, invocation.Args...)
	command.Stdin = streams.Input
	command.Stdout = streams.Output
	command.Stderr = streams.Errors
	return command.Run()
}
