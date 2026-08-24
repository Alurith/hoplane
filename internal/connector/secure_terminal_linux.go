//go:build linux

package connector

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func configureSecureTerminal(command *exec.Cmd, input io.Reader) (func() error, error) {
	file, ok := input.(*os.File)
	if !ok || !term.IsTerminal(int(file.Fd())) {
		return func() error { return nil }, nil
	}

	fd := int(file.Fd())
	processGroupID, err := unix.IoctlGetInt(fd, unix.TIOCGPGRP)
	if err != nil {
		return nil, fmt.Errorf("read terminal process group: %w", err)
	}
	attributes := command.SysProcAttr
	if attributes == nil {
		return nil, fmt.Errorf("secure process attributes are unavailable")
	}
	attributes.Foreground = true
	attributes.Ctty = fd
	return func() error {
		signal.Ignore(syscall.SIGTTOU)
		defer signal.Reset(syscall.SIGTTOU)
		return unix.IoctlSetPointerInt(fd, unix.TIOCSPGRP, processGroupID)
	}, nil
}
