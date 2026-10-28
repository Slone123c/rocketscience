//go:build darwin

package core

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

func clearQuarantineAttr(paths ...string) {
	attr, err := syscall.BytePtrFromString("com.apple.quarantine")
	if err != nil {
		return
	}
	for _, path := range paths {
		if path == "" {
			continue
		}
		p, err := syscall.BytePtrFromString(path)
		if err != nil {
			continue
		}
		_, _, _ = syscall.Syscall(syscall.SYS_REMOVEXATTR, uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(attr)), 0)
	}
}

// tunLauncher owns the unprivileged osascript process and a private watchdog file.
// The privileged supervisor terminates mihomo when this file goes stale.
type tunLauncher struct {
	cmd    *exec.Cmd
	dir    string
	done   chan struct{}
	exited chan struct{}
	once   sync.Once
}

func shellArgument(value string) (string, error) {
	if strings.ContainsAny(value, "\x00\r\n") {
		return "", errors.New("路径包含不支持的控制字符")
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'", nil
}
func tunSupervisorPath() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	for _, path := range []string{filepath.Join(filepath.Dir(executable), "..", "Resources", "bin", "rs-tun-supervisor"), filepath.Join("resources", "bin", "rs-tun-supervisor"), filepath.Join("..", "..", "resources", "bin", "rs-tun-supervisor")} {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return filepath.Abs(path)
		}
	}
	return "", errors.New("未找到 TUN 辅助程序，请重新构建应用")
}
func launchTun(core, dir string) (*tunLauncher, error) {
	supervisor, err := tunSupervisorPath()
	if err != nil {
		return nil, err
	}
	core, err = filepath.Abs(core)
	if err != nil {
		return nil, err
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	clearQuarantineAttr(supervisor, core)
	arguments := []string{supervisor, core, dir, strconv.Itoa(os.Getuid())}
	quoted := make([]string, len(arguments))
	for i, arg := range arguments {
		quoted[i], err = shellArgument(arg)
		if err != nil {
			return nil, err
		}
	}
	for _, name := range []string{"heartbeat", "tun-core.log", "core.pid"} {
		if err = os.WriteFile(filepath.Join(dir, name), []byte("running\n"), 0600); err != nil {
			return nil, err
		}
	}
	script := "do shell script " + strconv.Quote(strings.Join(quoted, " ")) + " with administrator privileges"
	cmd := exec.Command("/usr/bin/osascript", "-e", script)
	launcher := &tunLauncher{cmd: cmd, dir: dir, done: make(chan struct{}), exited: make(chan struct{})}
	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("无法请求 TUN 授权：%w", err)
	}
	go launcher.heartbeat()
	return launcher, nil
}

// launchUnprivileged uses the same watchdog without an authorization prompt.
func launchUnprivileged(core, dir string) (*tunLauncher, error) {
	supervisor, err := tunSupervisorPath()
	if err != nil {
		return nil, err
	}
	core, err = filepath.Abs(core)
	if err != nil {
		return nil, err
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	clearQuarantineAttr(supervisor, core)
	for _, name := range []string{"heartbeat", "tun-core.log", "core.pid"} {
		if err = os.WriteFile(filepath.Join(dir, name), []byte("running\n"), 0600); err != nil {
			return nil, err
		}
	}
	cmd := exec.Command(supervisor, core, dir, strconv.Itoa(os.Getuid()))
	launcher := &tunLauncher{cmd: cmd, dir: dir, done: make(chan struct{}), exited: make(chan struct{})}
	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("无法启动内核看门狗：%w", err)
	}
	go launcher.heartbeat()
	return launcher, nil
}
func (l *tunLauncher) heartbeat() {
	defer close(l.exited)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-l.done:
			return
		case <-ticker.C:
			if err := os.WriteFile(filepath.Join(l.dir, "heartbeat"), []byte("running\n"), 0600); err != nil {
				return
			}
		}
	}
}
func (l *tunLauncher) stop() error {
	l.once.Do(func() { close(l.done) })
	<-l.exited
	if err := os.WriteFile(filepath.Join(l.dir, "heartbeat"), []byte("stop\n"), 0600); err != nil {
		return fmt.Errorf("无法通知 TUN 停止：%w", err)
	}
	return nil
}
func (l *tunLauncher) finished() { l.once.Do(func() { close(l.done) }); <-l.exited }
