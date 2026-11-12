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
	original, err := os.Lstat(runtime)
	if err != nil {
		return err
	}
	if !original.IsDir() || original.Mode()&os.ModeSymlink != 0 {
		return errors.New("GEO 临时目录不能为符号链接")
	}
	root, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return err
	}
	actual, err := filepath.EvalSymlinks(runtime)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, actual)
	if err != nil || filepath.Dir(rel) != "." || !strings.HasPrefix(filepath.Base(rel), "rocketscience-core-") {
		return errors.New("GEO 临时目录不在预期位置")
	}
	info, err := os.Lstat(actual)
	if err != nil {
		return err
	}
	p, err := windows.UTF16PtrFromString(actual)
	if err != nil {
		return err
	}
	attrs, err := windows.GetFileAttributes(p)
	if err != nil {
		return err
	}
	if attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return errors.New("GEO 临时目录不能为重解析点")
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return errors.New("GEO 临时目录权限无效")
	}
	return nil
}

func processAlive(pid int) bool {
	if pid <= 1 {
		return true
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	_ = windows.CloseHandle(h)
	return true
}
