//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package safeio

import "os"

func ownerMatchesCurrentUser(string, os.FileInfo) (bool, bool) {
	return false, false
}
