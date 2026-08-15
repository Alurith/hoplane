package safeio

import (
	"fmt"
	"os"
	"path/filepath"
)

// SnapshotFile copies a validated regular file into a private temporary
// directory. The returned path is stable for the lifetime of the cleanup
// function, which is useful when a child process will reopen a pathname.
func SnapshotFile(path string, policy Policy) (string, func(), error) {
	contents, err := ReadFile(path, policy)
	if err != nil {
		return "", nil, err
	}
	directory, err := os.MkdirTemp("", "hoplane-input-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(directory) }
	if err := EnsurePrivateDirectory(directory); err != nil {
		cleanup()
		return "", nil, err
	}
	filePath := filepath.Join(directory, filepath.Base(path))
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	if _, err := file.Write(contents); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, err
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := Validate(filePath, policy); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("validate snapshot: %w", err)
	}
	return filePath, cleanup, nil
}
