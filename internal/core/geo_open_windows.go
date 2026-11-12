//go:build windows

package core

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func openGeoSourceFile(path string) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	attrs, err := windows.GetFileAttributes(p)
	if err != nil {
		return nil, fmt.Errorf("读取 GEO 数据失败：%w", err)
	}
	if attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return nil, errors.New("GEO 数据不能为重解析点")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("读取 GEO 数据失败：%w", err)
	}
	return f, nil
}
