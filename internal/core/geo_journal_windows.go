//go:build windows

package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func validGeoRuntime(runtime string) error {
	handle, err := createFileNoFollow(runtime, windows.READ_CONTROL|windows.FILE_READ_ATTRIBUTES, true)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	reparse, err := handleIsReparsePoint(handle)
	if err != nil {
		return err
	}
	if reparse {
		return errors.New("GEO 临时目录不能为重解析点")
	}
	isDir, err := handleIsDirectory(handle)
	if err != nil {
		return err
	}
	if !isDir {
		return errors.New("GEO 临时目录不在预期位置")
	}
	finalPath, err := normalizedFinalPath(handle)
	if err != nil {
		return err
	}
	tempHandle, err := createFileNoFollow(os.TempDir(), windows.FILE_READ_ATTRIBUTES, true)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(tempHandle)
	tempRoot, err := normalizedFinalPath(tempHandle)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(tempRoot, finalPath)
	if err != nil || filepath.Dir(rel) != "." || !strings.HasPrefix(filepath.Base(rel), "rocketscience-core-") {
		return errors.New("GEO 临时目录不在预期位置")
	}
	if err := validatePrivateDirectorySecurity(handle); err != nil {
		return err
	}
	return nil
}

func processAlive(pid int) bool {
	if pid <= 1 {
		return true
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		if errno, ok := err.(windows.Errno); ok && errno == windows.ERROR_ACCESS_DENIED {
			return true
		}
		return false
	}
	defer windows.CloseHandle(handle)
	var code uint32
	if err := windows.GetExitCodeProcess(handle, &code); err != nil {
		return true
	}
	return code == stillActive
}
