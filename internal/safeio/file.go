// Package safeio contains conservative filesystem checks for configuration
// files and other local inputs that influence process execution.
package safeio

import (
	"errors"
	"fmt"
	"io"
	"os"
)

var (
	ErrNotRegular        = errors.New("file is not a regular file")
	ErrUnsafePermissions = errors.New("file permissions are too broad")
	ErrWrongOwner        = errors.New("file is not owned by the current user")
	ErrOwnerUnavailable  = errors.New("file ownership cannot be verified")
	ErrTooLarge          = errors.New("file exceeds the configured size limit")
)

type Policy struct {
	MaxBytes              int64
	RequireOwner          bool
	RejectGroupOtherWrite bool
	RequirePrivate        bool
}

// Validate checks a path without opening it. Symlinks, special files and
// files that do not satisfy the policy are rejected.
func Validate(path string, policy Policy) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	return validateInfo(path, info, policy)
}

// EnsurePrivateDirectory makes an owner-controlled directory private.
func EnsurePrivateDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%q: path is not a directory", path)
	}
	owned, available := ownerMatchesCurrentUser(path, info)
	if !available {
		return fmt.Errorf("%q: %w", path, ErrOwnerUnavailable)
	}
	if !owned {
		return fmt.Errorf("%q: %w", path, ErrWrongOwner)
	}
	if err := validatePermissions(path, info, Policy{RejectGroupOtherWrite: true}); err != nil {
		return err
	}
	return ValidateDirectory(path, Policy{RequireOwner: true, RejectGroupOtherWrite: true})
}

// ValidateDirectory checks a directory and its ownership/permissions.
func ValidateDirectory(path string, policy Policy) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	return validateDirectoryInfo(path, info, policy)
}

func validateDirectoryInfo(path string, info os.FileInfo, policy Policy) error {
	if !info.IsDir() {
		return fmt.Errorf("%q: path is not a directory", path)
	}
	if err := validatePermissions(path, info, policy); err != nil {
		return err
	}
	if policy.RequireOwner {
		owned, available := ownerMatchesCurrentUser(path, info)
		if !available {
			return fmt.Errorf("%q: %w", path, ErrOwnerUnavailable)
		}
		if !owned {
			return fmt.Errorf("%q: %w", path, ErrWrongOwner)
		}
	}
	return nil
}

// ReadFile reads a policy-compliant regular file with a hard byte limit.
func ReadFile(path string, policy Policy) ([]byte, error) {
	file, err := openPolicyFile(path, policy)
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck

	if policy.MaxBytes <= 0 {
		return io.ReadAll(file)
	}
	contents, err := io.ReadAll(io.LimitReader(file, policy.MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(contents)) > policy.MaxBytes {
		return nil, fmt.Errorf("%q: %w", path, ErrTooLarge)
	}
	return contents, nil
}

func validateInfo(path string, info os.FileInfo, policy Policy) error {
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%q: %w", path, ErrNotRegular)
	}
	if policy.MaxBytes > 0 && info.Size() > policy.MaxBytes {
		return fmt.Errorf("%q: %w", path, ErrTooLarge)
	}
	if err := validatePermissions(path, info, policy); err != nil {
		return err
	}
	if policy.RequireOwner {
		owned, available := ownerMatchesCurrentUser(path, info)
		if !available {
			return fmt.Errorf("%q: %w", path, ErrOwnerUnavailable)
		}
		if !owned {
			return fmt.Errorf("%q: %w", path, ErrWrongOwner)
		}
	}
	return nil
}
