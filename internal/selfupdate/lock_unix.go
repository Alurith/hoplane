//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package selfupdate

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

func acquireUpdateLock(ctx context.Context, path string) (func(), error) {
	lockPath := filepath.Clean(path + ".update.lock")
	fd, err := syscall.Open(lockPath, syscall.O_RDWR|syscall.O_CREAT|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open update lock: %w", err)
	}
	file := os.NewFile(uintptr(fd), lockPath)
	if file == nil {
		_ = syscall.Close(fd)
		return nil, fmt.Errorf("open update lock: %w", os.ErrInvalid)
	}
	release := func() {
		_ = syscall.Flock(fd, syscall.LOCK_UN)
		_ = file.Close()
	}
	info, err := file.Stat()
	if err != nil {
		release()
		return nil, fmt.Errorf("stat update lock: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		release()
		return nil, fmt.Errorf("update lock has unsafe type or permissions")
	}
	if err := safeio.Validate(lockPath, safeio.Policy{RequireOwner: true, RequirePrivate: true}); err != nil {
		release()
		return nil, fmt.Errorf("validate update lock: %w", err)
	}
	if err := ctx.Err(); err != nil {
		release()
		return nil, err
	}

	deadline := time.NewTimer(updateLockTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(updateLockPoll)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			release()
			return nil, err
		}
		err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			if contextErr := ctx.Err(); contextErr != nil {
				release()
				return nil, contextErr
			}
			return release, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			release()
			return nil, fmt.Errorf("lock update: %w", err)
		}
		select {
		case <-ctx.Done():
			release()
			return nil, ctx.Err()
		case <-deadline.C:
			release()
			return nil, fmt.Errorf("lock update: timeout after %s", updateLockTimeout)
		case <-ticker.C:
		}
	}
}
