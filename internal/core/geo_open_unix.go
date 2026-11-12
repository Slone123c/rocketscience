//go:build unix

package core

import (
	"fmt"
	"os"
	"syscall"
)

func openGeoSourceFile(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("读取 GEO 数据失败：%w", err)
	}
	return os.NewFile(uintptr(fd), path), nil
}
