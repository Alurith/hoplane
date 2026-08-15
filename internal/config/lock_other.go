//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package config

import (
	"context"
	"errors"
)

func acquireConfigLock(context.Context, string) (func(), error) {
	return nil, errors.New("configuration locking is unsupported on this platform")
}
