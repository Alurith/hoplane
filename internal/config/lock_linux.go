//go:build linux

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
	fd, err := syscall.Open(
		lockPath,
		syscall.O_RDWR|syscall.O_CREAT|syscall.O_CLOEXEC|syscall.O_NOFOLLOW,
		0o600,
	)
	if err != nil {
		return nil, fmt.Errorf("open config lock: %w", err)
	}
	file := os.NewFile(uintptr(fd), lockPath)
	if file == nil {
		_ = syscall.Close(fd)
		return nil, fmt.Errorf("open config lock: %w", os.ErrInvalid)
	}
	closeFile := func() { _ = file.Close() }

	info, err := file.Stat()
	if err != nil {
		closeFile()
		return nil, fmt.Errorf("stat config lock: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		closeFile()
		return nil, fmt.Errorf("config lock has unsafe type or permissions")
	}
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
		err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			if contextErr := ctx.Err(); contextErr != nil {
				_ = syscall.Flock(fd, syscall.LOCK_UN)
				closeFile()
				return nil, contextErr
			}
			return closeFile, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
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
