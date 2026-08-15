//go:build windows

package safeio

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func ownerMatchesCurrentUser(path string, _ os.FileInfo) (bool, bool) {
	descriptor, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil || descriptor == nil {
		return false, false
	}
	return descriptorIsSafe(descriptor, true, true)
}

func validateOpenedHandle(file *os.File, requireCurrentOwner bool) error {
	descriptor, err := windows.GetSecurityInfo(
		windows.Handle(file.Fd()),
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return fmt.Errorf("read opened security descriptor: %w", err)
	}
	if descriptor == nil {
		return errors.New("opened file has no security descriptor")
	}
	safe, available := descriptorIsSafe(descriptor, requireCurrentOwner, true)
	if !available || !safe {
		return errors.New("opened file has unsafe owner or DACL")
	}
	return nil
}

func descriptorIsSafe(descriptor *windows.SECURITY_DESCRIPTOR, requireCurrentOwner, private bool) (bool, bool) {
	owner, _, err := descriptor.Owner()
	if err != nil || owner == nil {
		return false, false
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil {
		return false, false
	}
	if requireCurrentOwner && !owner.Equals(user.User.Sid) {
		return false, true
	}

	acl, present, err := descriptor.DACL()
	if err != nil || !present || acl == nil {
		return false, true
	}
	trusted := []*windows.SID{owner, user.User.Sid}
	for _, sidType := range []windows.WELL_KNOWN_SID_TYPE{
		windows.WinLocalSystemSid,
		windows.WinBuiltinAdministratorsSid,
	} {
		sid, err := windows.CreateWellKnownSid(sidType)
		if err != nil {
			return false, false
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
			return false, false
		}
		if ace.Header.AceType == windows.ACCESS_DENIED_ACE_TYPE {
			continue
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			return false, true
		}
		sid := (*windows.SID)(unsafe.Pointer(uintptr(unsafe.Pointer(ace)) + unsafe.Offsetof(windows.ACCESS_ALLOWED_ACE{}.SidStart)))
		if private {
			if uint32(ace.Mask) == 0 {
				continue
			}
		} else if uint32(ace.Mask)&writeMask == 0 {
			continue
		}
		trustedSID := false
		for _, allowed := range trusted {
			if sid.Equals(allowed) {
				trustedSID = true
				break
			}
		}
		if !trustedSID {
			return false, true
		}
	}
	return true, true
}
