//go:build windows

package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Alurith/hoplane/internal/safeio"
	"golang.org/x/sys/windows"
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

	var overlapped windows.Overlapped
	locked := false
	release := func() {
		if locked {
			_ = windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, &overlapped)
		}
		closeFile()
	}
	deadline := time.NewTimer(configLockTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(configLockPoll)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			release()
			return nil, err
		}
		err := windows.LockFileEx(
			windows.Handle(file.Fd()),
			windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
			0,
			1,
			0,
			&overlapped,
		)
		if err == nil {
			locked = true
			if contextErr := ctx.Err(); contextErr != nil {
				release()
				return nil, contextErr
			}
			return release, nil
		}
		if !errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			release()
			return nil, fmt.Errorf("lock config: %w", err)
		}
		select {
		case <-ctx.Done():
			release()
			return nil, ctx.Err()
		case <-deadline.C:
			release()
			return nil, fmt.Errorf("lock config: timeout after %s", configLockTimeout)
		case <-ticker.C:
		}
	}
}
