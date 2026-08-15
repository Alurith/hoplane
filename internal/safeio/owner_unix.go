//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package safeio

import (
	"os"
	"syscall"
)

func ownerMatchesCurrentUser(_ string, info os.FileInfo) (bool, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false, false
	}
	return uint64(stat.Uid) == uint64(os.Geteuid()), true
}
