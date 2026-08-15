//go:build aix || darwin || dragonfly || freebsd || netbsd || openbsd || solaris

package connector

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func validateExecutable(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("executable path must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("executable is not a regular executable file")
	}
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("executable or its file is group/other writable")
	}
	if !ownedByRootOrCurrent(info) {
		return fmt.Errorf("executable is not owned by root or the current user")
	}

	for directory := filepath.Dir(path); ; directory = filepath.Dir(directory) {
		directoryInfo, err := os.Lstat(directory)
		if err != nil {
			return err
		}
		if !directoryInfo.IsDir() || directoryInfo.Mode().Perm()&0o022 != 0 {
			return fmt.Errorf("executable directory is unsafe")
		}
		if !ownedByRootOrCurrent(directoryInfo) {
			return fmt.Errorf("executable directory is not owned by root or the current user")
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			break
		}
	}
	return nil
}

func ownedByRootOrCurrent(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	uid := uint64(stat.Uid)
	current := uint64(os.Geteuid())
	return uid == 0 || uid == current
}
