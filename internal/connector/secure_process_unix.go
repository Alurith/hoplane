//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package connector

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"sync/atomic"
	"syscall"
)

func configureSecureCommand(command *exec.Cmd, input io.Reader) (func() error, error) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return configureSecureTerminal(command, input)
}

func secureProcessGroupID(command *exec.Cmd) int {
	if command.Process == nil {
		return 0
	}
	return command.Process.Pid
}

func terminateSecureCommand(command *exec.Cmd, groupID ...int) error {
	return signalSecureProcessGroup(command, syscall.SIGTERM, groupID...)
}

func killSecureCommand(command *exec.Cmd, groupID ...int) error {
	return signalSecureProcessGroup(command, syscall.SIGKILL, groupID...)
}

func signalSecureProcessGroup(command *exec.Cmd, signal syscall.Signal, groupID ...int) error {
	pid := secureProcessGroupID(command)
	if len(groupID) > 0 {
		pid = groupID[0]
	}
	if pid <= 1 {
		return nil
	}
	if err := syscall.Kill(-pid, signal); err == nil || err == syscall.ESRCH {
		return nil
	} else {
		return err
	}
}

func isArtificialSecretWriteError(err error) bool {
	return errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.EBADF) ||
		errors.Is(err, os.ErrClosed) || errors.Is(err, io.ErrClosedPipe)
}

func secureProcessGroupAlive(command *exec.Cmd, groupID int, _ *atomic.Bool) bool {
	pid := secureProcessGroupID(command)
	if groupID != 0 {
		pid = groupID
	}
	if pid <= 1 {
		return false
	}
	err := syscall.Kill(-pid, 0)
	return err == nil || err == syscall.EPERM
}
