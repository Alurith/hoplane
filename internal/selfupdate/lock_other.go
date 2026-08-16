//go:build !linux && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd && !windows

package selfupdate

import (
	"context"
	"errors"
)

func acquireUpdateLock(ctx context.Context, _ string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, errors.New("self-update locking is unsupported on this platform")
}
