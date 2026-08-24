//go:build windows || plan9 || js

package connector

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"sync/atomic"
)

func configureSecureCommand(*exec.Cmd, io.Reader) (func() error, error) {
	return func() error { return nil }, nil
}

func secureProcessGroupID(command *exec.Cmd) int {
	if command.Process == nil {
		return 0
	}
	return command.Process.Pid
}

func terminateSecureCommand(command *exec.Cmd, _ ...int) error {
	if command.Process == nil {
		return nil
	}
	return command.Process.Kill()
}

func killSecureCommand(command *exec.Cmd, _ ...int) error {
	if command.Process == nil {
		return nil
	}
	return command.Process.Kill()
}

func isArtificialSecretWriteError(err error) bool {
	return errors.Is(err, os.ErrClosed) || errors.Is(err, io.ErrClosedPipe)
}

// Non-Unix systems have no process-group guarantee here. The direct process
// is the only cleanup primitive available to this build.
func secureProcessGroupAlive(command *exec.Cmd, _ int, processDone *atomic.Bool) bool {
	return command.Process != nil && !processDone.Load()
}
