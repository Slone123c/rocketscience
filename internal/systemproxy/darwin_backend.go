//go:build darwin

package systemproxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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

// DarwinBackend talks to the signed SystemConfiguration helper. Writes share
// one helper process so the password granted on start also covers restore.
type DarwinBackend struct {
	Helper string

	mu      sync.Mutex
	session *exec.Cmd
	stdin   io.WriteCloser
	replies *bufio.Scanner
}

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
func (b *DarwinBackend) CompareAndSwap(changes []Change) ([]string, error) {
	in, err := json.Marshal(changes)
	if err != nil {
		return nil, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.ensureSession(); err != nil {
		return nil, err
	}
	if _, err := io.WriteString(b.stdin, string(in)+"\n"); err != nil {
		b.closeSession()
		return nil, fmt.Errorf("系统代理操作失败：%w", err)
	}
	if !b.replies.Scan() {
		b.closeSession()
		return nil, errors.New("系统代理操作失败：辅助程序已退出")
	}
	var result struct {
		Conflicts []string `json:"conflicts"`
	}
	if err = json.Unmarshal(b.replies.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("系统代理返回数据无效：%w", err)
	}
	return result.Conflicts, nil
}

func (b *DarwinBackend) CompareAndSwapPartial(changes []Change) ([]string, error) {
	return b.CompareAndSwap(changes)
}

// ensureSession starts the helper on the first write and keeps the
// authorization it obtained. Called with mu held.
func (b *DarwinBackend) ensureSession() error {
	if b.session != nil && b.session.ProcessState == nil {
		return nil
	}
	helper, err := b.helper()
	if err != nil {
		return err
	}
	clearQuarantineAttr(helper)
	cmd := exec.Command(helper, "cas")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("系统代理操作失败：%w", err)
	}
	b.session, b.stdin, b.replies = cmd, stdin, bufio.NewScanner(stdout)
	return nil
}

func (b *DarwinBackend) closeSession() {
	if b.stdin != nil {
		_ = b.stdin.Close()
	}
	if b.session != nil && b.session.Process != nil {
		_ = b.session.Wait()
	}
	b.session, b.stdin, b.replies = nil, nil, nil
}
