//go:build !linux && !windows

package safeio

import "os"

func openPolicyFile(path string, policy Policy) (*os.File, error) {
	if err := Validate(path, policy); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := validateInfo(path, info, policy); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}
