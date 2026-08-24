//go:build aix || solaris

package connector

import (
	"io"
	"os/exec"
)

func configureSecureTerminal(*exec.Cmd, io.Reader) (func() error, error) {
	return func() error { return nil }, nil
}
