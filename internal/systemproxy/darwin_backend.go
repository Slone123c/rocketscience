//go:build darwin

package systemproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

func clearQuarantineAttr(path string) {
	if path == "" {
		return
	}
	p, err := syscall.BytePtrFromString(path)
	if err != nil {
		return
	}
	attr, err := syscall.BytePtrFromString("com.apple.quarantine")
	if err != nil {
		return
	}
	_, _, _ = syscall.Syscall(syscall.SYS_REMOVEXATTR, uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(attr)), 0)
}

// DarwinBackend talks to the signed SystemConfiguration helper over standard input.
type DarwinBackend struct{ Helper string }

func (b DarwinBackend) helper() (string, error) {
	if b.Helper != "" {
		return b.Helper, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	candidates := []string{filepath.Join(filepath.Dir(exe), "..", "Resources", "bin", "rs-system-proxy"), filepath.Join("resources", "bin", "rs-system-proxy")}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path, nil
		}
	}
	return "", errors.New("找不到系统代理辅助程序，请重新构建应用")
}
func (b DarwinBackend) run(mode string, input []byte) ([]byte, error) {
	helper, err := b.helper()
	if err != nil {
		return nil, err
	}
	clearQuarantineAttr(helper)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, helper, mode)
	cmd.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, errors.New("系统代理操作超时")
	}
	if err != nil {
		return nil, fmt.Errorf("系统代理操作失败：%s", strings.TrimSpace(stderr.String()))
	}
	return out, nil
}
func (b DarwinBackend) Snapshot() ([]Service, error) {
	out, err := b.run("snapshot", nil)
	if err != nil {
		return nil, err
	}
	var services []Service
	if err = json.Unmarshal(out, &services); err != nil {
		return nil, fmt.Errorf("系统代理状态无效：%w", err)
	}
	seen := map[string]bool{}
	filtered := make([]Service, 0, len(services))
	for _, s := range services {
		if s.ID == "" || seen[s.ID] {
			continue
		}
		seen[s.ID] = true
		filtered = append(filtered, s)
	}
	return filtered, nil
}
func (b DarwinBackend) CompareAndSwap(changes []Change) ([]string, error) {
	in, err := json.Marshal(changes)
	if err != nil {
		return nil, err
	}
	out, err := b.run("cas", in)
	if err != nil {
		return nil, err
	}
	var result struct {
		Conflicts []string `json:"conflicts"`
	}
	if err = json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("系统代理返回数据无效：%w", err)
	}
	return result.Conflicts, nil
}
