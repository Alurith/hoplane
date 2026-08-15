package safeio

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ValidateAncestors rejects symlinked path components and writable ancestors.
// Sticky world-writable directories such as /tmp are allowed because they do
// not permit replacing another user's directory entry.
func ValidateAncestors(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	current := filepath.Dir(absolute)
	for {
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			parent := filepath.Dir(current)
			if parent == current {
				return nil
			}
			current = parent
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("%q: unsafe path ancestor", current)
		}
		if err := validateAncestorPermissions(current, info); err != nil {
			return err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
		current = parent
	}
}
