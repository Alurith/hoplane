//go:build windows

package safeio

import "os"

func validateAncestorPermissions(string, os.FileInfo) error {
	return nil
}

func validatePermissions(string, os.FileInfo, Policy) error {
	// Windows ACLs are checked by owner_windows.go; POSIX mode bits are not
	// meaningful on this platform.
	return nil
}
