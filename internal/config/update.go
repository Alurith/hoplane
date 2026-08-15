package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Alurith/hoplane/internal/safeio"
)

// Update serializes a read-modify-write transaction for one configuration
// path. The lock is held through validation and the atomic replacement.
func Update(ctx context.Context, path string, mutate func(*File) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if mutate == nil {
		return fmt.Errorf("config mutation cannot be nil")
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
}
