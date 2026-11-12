//go:build windows

package core

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func openGeoSourceFile(path string) (*os.File, error) {
	handle, err := createFileNoFollow(path, windows.GENERIC_READ, false)
	if err != nil {
		return nil, fmt.Errorf("读取 GEO 数据失败：%w", err)
	}
	reparse, err := handleIsReparsePoint(handle)
	if err != nil {
		windows.CloseHandle(handle)
		return nil, fmt.Errorf("读取 GEO 数据失败：%w", err)
	}
	if reparse {
		windows.CloseHandle(handle)
		return nil, errors.New("GEO 数据不能为重解析点")
	}
	dir, err := handleIsDirectory(handle)
	if err != nil {
		windows.CloseHandle(handle)
		return nil, fmt.Errorf("读取 GEO 数据失败：%w", err)
	}
	if dir {
		windows.CloseHandle(handle)
		return nil, errors.New("GEO 数据不是普通文件")
	}
	return os.NewFile(uintptr(handle), path), nil
}
