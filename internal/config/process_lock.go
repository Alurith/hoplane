package config

import (
	"context"
	"path/filepath"
	"sync"
)

type processLock struct {
	ready chan struct{}
}

var processLocks = struct {
	sync.Mutex
	values map[string]*processLock
}{values: make(map[string]*processLock)}

func acquireProcessLock(ctx context.Context, path string) (func(), error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	key := absolute
	if directory, resolveErr := filepath.EvalSymlinks(filepath.Dir(absolute)); resolveErr == nil {
		key = filepath.Join(directory, filepath.Base(absolute))
	}
	processLocks.Lock()
	lock := processLocks.values[key]
	if lock == nil {
		lock = &processLock{ready: make(chan struct{}, 1)}
		lock.ready <- struct{}{}
		processLocks.values[key] = lock
	}
	processLocks.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-lock.ready:
		return func() { lock.ready <- struct{}{} }, nil
	}
}
