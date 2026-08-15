//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package safeio

import (
	"fmt"
	"os"
)

func validateAncestorPermissions(path string, info os.FileInfo) error {
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("%q: writable path ancestor", path)
	}
	return nil
}

func validatePermissions(path string, info os.FileInfo, policy Policy) error {
	permissions := info.Mode().Perm()
	if policy.RequirePrivate && permissions&0o077 != 0 {
		return fmt.Errorf("%q: %w (want owner-only permissions)", path, ErrUnsafePermissions)
	}
	if policy.RejectGroupOtherWrite && permissions&0o022 != 0 {
		return fmt.Errorf("%q: %w (group/other write bit set)", path, ErrUnsafePermissions)
	}
	return nil
}
