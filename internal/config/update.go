package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Alurith/hoplane/internal/safeio"
)

// WithLock runs an operation while holding the same private configuration lock
// used by read-modify-write updates. It is used for side effects keyed by a
// connection ID, such as keyring changes, so delete/set cannot race.
func WithLock(ctx context.Context, path string, action func() error) error {
	if action == nil {
		return fmt.Errorf("locked configuration action cannot be nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := safeio.ValidateAncestors(path); err != nil {
		return fmt.Errorf("validate config path: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := safeio.EnsurePrivateDirectory(directory); err != nil {
		return fmt.Errorf("validate config directory: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	releaseProcessLock, err := acquireProcessLock(ctx, path)
	if err != nil {
		return err
	}
	defer releaseProcessLock()

	release, err := acquireConfigLock(ctx, path)
	if err != nil {
		return err
	}
	defer release()

	if err := ctx.Err(); err != nil {
		return err
	}
	return action()
}

// Update serializes a read-modify-write transaction for one configuration
// path. The lock is held through validation and the atomic replacement.
func Update(ctx context.Context, path string, mutate func(*File) error) error {
	if mutate == nil {
		return fmt.Errorf("config mutation cannot be nil")
	}
	return WithLock(ctx, path, func() error {
		file, err := Load(path)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := mutate(&file); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return Save(path, file)
	})
}
