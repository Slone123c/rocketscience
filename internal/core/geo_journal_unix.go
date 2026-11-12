//go:build unix

package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(st.Uid) != os.Getuid() || !info.IsDir() || info.Mode().Perm() != 0700 {
		return errors.New("GEO 临时目录权限无效")
	}
	return nil
}

func processAlive(pid int) bool {
	if pid <= 1 {
		return true
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
