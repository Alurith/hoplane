//go:build aix || solaris

package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Alurith/hoplane/internal/safeio"
)

func acquireConfigLock(ctx context.Context, path string) (func(), error) {
	lockPath := filepath.Clean(path + ".lock")
	file, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open config lock: %w", err)
	}
	closeFile := func() { _ = file.Close() }
	if err := safeio.Validate(lockPath, safeio.Policy{RequireOwner: true, RequirePrivate: true}); err != nil {
		closeFile()
		return nil, fmt.Errorf("validate config lock: %w", err)
	}
	if err := ctx.Err(); err != nil {
		closeFile()
		return nil, err
	}

	deadline := time.NewTimer(configLockTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(configLockPoll)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			closeFile()
			return nil, err
		}
		lock := syscall.Flock_t{Type: syscall.F_WRLCK, Whence: 0, Start: 0, Len: 0}
		err := syscall.FcntlFlock(file.Fd(), syscall.F_SETLK, &lock)
		if err == nil {
			if contextErr := ctx.Err(); contextErr != nil {
				_ = unlockConfigFile(file)
				closeFile()
				return nil, contextErr
			}
			return func() {
				_ = unlockConfigFile(file)
				closeFile()
			}, nil
		}
		if !errors.Is(err, syscall.EAGAIN) && !errors.Is(err, syscall.EACCES) {
			closeFile()
			return nil, fmt.Errorf("lock config: %w", err)
		}
		select {
		case <-ctx.Done():
			closeFile()
			return nil, ctx.Err()
		case <-deadline.C:
			closeFile()
			return nil, fmt.Errorf("lock config: timeout after %s", configLockTimeout)
		case <-ticker.C:
		}
	}
}

func unlockConfigFile(file *os.File) error {
	lock := syscall.Flock_t{Type: syscall.F_UNLCK, Whence: 0, Start: 0, Len: 0}
	return syscall.FcntlFlock(file.Fd(), syscall.F_SETLK, &lock)
}
