//go:build windows

package selfupdate

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

func acquireUpdateLock(ctx context.Context, path string) (func(), error) {
	lockPath := filepath.Clean(path + ".update.lock")
	file, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open update lock: %w", err)
	}
	var overlapped windows.Overlapped
	locked := false
	release := func() {
		if locked {
			_ = windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, &overlapped)
		}
		_ = file.Close()
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
