//go:build windows

package core

import (
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const stillActive = 259

func createFileNoFollow(path string, access uint32, directory bool) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	attrs := uint32(windows.FILE_ATTRIBUTE_NORMAL | windows.FILE_FLAG_OPEN_REPARSE_POINT)
	if directory {
		attrs = windows.FILE_FLAG_BACKUP_SEMANTICS | windows.FILE_FLAG_OPEN_REPARSE_POINT
	}
	handle, err := windows.CreateFile(p, access, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, attrs, 0)
	if err != nil {
		return 0, err
	}
	return handle, nil
}

func fileInfoByHandle(handle windows.Handle) (windows.ByHandleFileInformation, error) {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return info, err
	}
	return info, nil
}

func handleIsReparsePoint(handle windows.Handle) (bool, error) {
	info, err := fileInfoByHandle(handle)
	if err != nil {
		return false, err
	}
	return info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0, nil
}

func handleIsDirectory(handle windows.Handle) (bool, error) {
	info, err := fileInfoByHandle(handle)
	if err != nil {
		return false, err
	}
	return info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0, nil
}

func normalizedFinalPath(handle windows.Handle) (string, error) {
	buf := make([]uint16, windows.MAX_PATH)
	for {
		n, err := windows.GetFinalPathNameByHandle(handle, &buf[0], uint32(len(buf)), 0)
		if err != nil {
			return "", err
		}
		if int(n) < len(buf) {
			path := windows.UTF16ToString(buf[:n])
			if strings.HasPrefix(path, `\\?\UNC\`) {
				path = `\\` + path[8:]
			} else if strings.HasPrefix(path, `\\?\`) {
				path = path[4:]
			}
			return strings.TrimRight(path, `\`), nil
		}
		buf = make([]uint16, n+1)
	}
}

func currentUserSID() (*windows.SID, error) {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return nil, err
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return nil, err
	}
	return user.User.Sid, nil
}

func validatePrivateDirectorySecurity(handle windows.Handle) error {
	sd, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("GEO 临时目录权限无效：%w", err)
	}
	self, err := currentUserSID()
	if err != nil {
		return err
	}
	trustedWriters, err := trustedWriterSIDs()
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil {
		return fmt.Errorf("GEO 临时目录权限无效：owner=%v err=%v", owner, err)
	}
	if !owner.Equals(self) && !trustedContains(trustedWriters, owner) {
		return fmt.Errorf("GEO 临时目录权限无效：owner=%s self=%s", owner.String(), self.String())
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	if dacl == nil {
		return fmt.Errorf("GEO 临时目录权限无效：dacl=nil")
	}
	writeMask := windows.ACCESS_MASK(windows.FILE_WRITE_DATA | windows.FILE_APPEND_DATA | windows.DELETE | windows.WRITE_DAC | windows.WRITE_OWNER | windows.GENERIC_WRITE | windows.GENERIC_ALL)
	var offending []string
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			continue
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			continue
		}
		if ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 {
			continue
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if self.Equals(sid) || owner.Equals(sid) || trustedContains(trustedWriters, sid) {
			continue
		}
		if ace.Mask&writeMask != 0 {
			offending = append(offending, fmt.Sprintf("sid=%s mask=%#x flags=%#x", sid.String(), uint32(ace.Mask), ace.Header.AceFlags))
		}
	}
	if len(offending) > 0 {
		return fmt.Errorf("GEO 临时目录权限无效：owner=%s self=%s offending=[%s]", owner.String(), self.String(), strings.Join(offending, ", "))
	}
	return nil
}

func trustedWriterSIDs() ([]*windows.SID, error) {
	var out []*windows.SID
	for _, spec := range []string{"S-1-5-18", "S-1-5-32-544"} {
		sid, err := windows.StringToSid(spec)
		if err != nil {
			return nil, err
		}
		out = append(out, sid)
	}
	return out, nil
}

func trustedContains(trusted []*windows.SID, sid *windows.SID) bool {
	for _, t := range trusted {
		if t.Equals(sid) {
			return true
		}
	}
	return false
}
