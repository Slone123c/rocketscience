//go:build darwin

package core

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"rocketscience/internal/corehook"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// The supervisor normally waits for its child. If it died unexpectedly, stop
// only a process whose command still matches this session's private directory.
func cleanupOrdinaryOrphan(dir, core string) error {
	raw, err := os.ReadFile(filepath.Join(dir, "core.pid"))
	if err != nil {
		return errors.New("缺少内核进程记录")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 1 {
		return errors.New("内核进程记录尚未就绪")
	}
	if !processAlive(pid) {
		return nil
	}
	resolved, err := filepath.EvalSymlinks(core)
	if err != nil {
		return err
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	expected := resolved + " -d " + dir + " -f " + filepath.Join(dir, "config.yaml")
	expectedWithHook := expected + " -post-up " + corehook.Command(dir)
	matches := func() (bool, error) {
		out, err := exec.Command("/bin/ps", "-ww", "-p", strconv.Itoa(pid), "-o", "command=").Output()
		if err != nil {
			if !processAlive(pid) {
				return false, nil
			}
			return false, err
		}
		command := strings.TrimSpace(string(out))
		return command == expected || command == expectedWithHook, nil
	}
	owned, err := matches()
	if err != nil {
		return err
	}
	if !owned {
		if !processAlive(pid) {
			return nil
		}
		return errors.New("进程标识已变化，未发送停止信号")
	}
	if err = syscall.Kill(pid, syscall.SIGINT); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	owned, err = matches()
	if err != nil {
		return err
	}
	if !owned {
		if !processAlive(pid) {
			return nil
		}
		return errors.New("进程标识已变化，未强制停止")
	}
	if err = syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("内核进程 %d 未退出", pid)
}
