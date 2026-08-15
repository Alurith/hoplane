//go:build windows

package connector

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func validateExecutable(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("executable path must be absolute")
	}
	if !strings.EqualFold(filepath.Ext(path), ".exe") {
		return fmt.Errorf("executable must be an .exe file")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("executable is not a regular file")
	}
	if err := validateWindowsACL(path); err != nil {
		return fmt.Errorf("validate executable ACL: %w", err)
	}
	for directory := filepath.Dir(path); ; directory = filepath.Dir(directory) {
		info, err := os.Lstat(directory)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("executable directory is unsafe")
		}
		if err := validateWindowsACL(directory); err != nil {
			return fmt.Errorf("validate executable directory ACL: %w", err)
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			break
		}
	}
	return nil
}

func validateWindowsACL(path string) error {
	descriptor, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return fmt.Errorf("read security descriptor: %w", err)
	}
	if descriptor == nil {
		return errors.New("missing security descriptor")
	}
	owner, _, err := descriptor.Owner()
	if err != nil {
		return err
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil || owner == nil {
		if err != nil {
			return fmt.Errorf("resolve current Windows user: %w", err)
		}
		return errors.New("resolve current Windows user")
	}
	if !trustedWindowsOwner(owner, user.User.Sid) {
		return fmt.Errorf("owner is not trusted")
	}
	acl, present, err := descriptor.DACL()
	if err != nil || !present || acl == nil {
		return fmt.Errorf("missing restrictive DACL")
	}
	trusted := []*windows.SID{owner, user.User.Sid}
	for _, sidType := range []windows.WELL_KNOWN_SID_TYPE{
		windows.WinLocalSystemSid,
		windows.WinBuiltinAdministratorsSid,
	} {
		sid, err := windows.CreateWellKnownSid(sidType)
		if err != nil {
			return err
		}
		trusted = append(trusted, sid)
	}
	const writeMask = uint32(
		windows.GENERIC_WRITE |
			windows.GENERIC_ALL |
			windows.FILE_WRITE_DATA |
			windows.FILE_APPEND_DATA |
			windows.FILE_WRITE_EA |
			windows.FILE_WRITE_ATTRIBUTES |
			0x00000040 |
			windows.DELETE |
			windows.WRITE_DAC |
			windows.WRITE_OWNER,
	)
	for index := uint32(0); index < uint32(acl.AceCount); index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, index, &ace); err != nil || ace == nil {
			return errors.New("read DACL entry")
		}
		if ace.Header.AceType == windows.ACCESS_DENIED_ACE_TYPE {
			continue
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			return errors.New("unsupported allowing DACL entry")
		}
		sid := (*windows.SID)(unsafe.Pointer(uintptr(unsafe.Pointer(ace)) + unsafe.Offsetof(windows.ACCESS_ALLOWED_ACE{}.SidStart)))
		if uint32(ace.Mask)&writeMask == 0 {
			continue
		}
		allowed := false
		for _, trustedSID := range trusted {
			if sid.Equals(trustedSID) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("DACL grants write access to an untrusted principal")
		}
	}
	return nil
}

func trustedWindowsOwner(owner, current *windows.SID) bool {
	if owner.Equals(current) || owner.String() == "S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464" {
		return true
	}
	for _, sidType := range []windows.WELL_KNOWN_SID_TYPE{
		windows.WinLocalSystemSid,
		windows.WinBuiltinAdministratorsSid,
	} {
		sid, err := windows.CreateWellKnownSid(sidType)
		if err == nil && owner.Equals(sid) {
			return true
		}
	}
	return false
}
