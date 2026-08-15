//go:build windows

package safeio

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func OpenDirectory(path string, policy Policy) (*os.File, error) {
	if err := ValidateDirectory(path, policy); err != nil {
		return nil, err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(
		name,
		windows.FILE_LIST_DIRECTORY|windows.READ_CONTROL,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		_ = windows.CloseHandle(handle)
		return nil, os.ErrInvalid
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := validateDirectoryInfo(path, info, policy); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("validate opened directory: %w", err)
	}
	if err := validateOpenedHandle(file, policy.RequireOwner); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}
