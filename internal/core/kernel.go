package core

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type KernelInfo struct {
	Available bool   `json:"available"`
	Path      string `json:"path"`
	Version   string `json:"version"`
	Platform  string `json:"platform"`
}

// KernelInfo works while stopped and never starts a controller or takes over traffic.
func (s *Service) KernelInfo() (KernelInfo, error) {
	info := KernelInfo{Path: corePath(), Platform: runtime.GOOS + "/" + runtime.GOARCH}
	if info.Path == "" {
		return info, errors.New("未找到 mihomo，请重新构建或安装应用")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, info.Path, "-v").Output()
	if err != nil {
		return info, fmt.Errorf("无法读取 mihomo 版本：%w", err)
	}
	version := strings.TrimSpace(string(output))
	if !strings.HasPrefix(version, "Mihomo ") {
		return info, errors.New("内核未返回有效的 mihomo 版本信息")
	}
	info.Available, info.Version = true, version
	return info, nil
}

// Restart uses the supervised lifecycle rather than /restart, which would bypass
// the application's process ownership and network restoration guarantees.
func (s *Service) Restart() error {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	s.mu.Lock()
	if s.cmd == nil || s.stopping {
		s.mu.Unlock()
		return errors.New("内核未运行或正在停止，无法重启")
	}
	var cfg struct {
		Mode string `json:"mode"`
	}
	var groups struct {
		Proxies map[string]Proxy `json:"proxies"`
	}
	if err := s.get("/configs", &cfg); err != nil {
		s.mu.Unlock()
		return fmt.Errorf("读取当前模式失败，未重启：%w", err)
	}
	if err := s.get("/proxies", &groups); err != nil {
		s.mu.Unlock()
		return fmt.Errorf("读取当前节点失败，未重启：%w", err)
	}
	content, port, bypass, lan, options := s.currentContent, s.mixedPort, cloneBypass(s.proxyBypass), s.lan, s.coreOptions
	s.mu.Unlock()
	var document map[string]any
	if err := yaml.Unmarshal([]byte(content), &document); err != nil {
		return err
	}
	document["mode"] = cfg.Mode
	updated, err := yaml.Marshal(document)
	if err != nil {
		return err
	}
	selected := make(map[string]string)
	for name, group := range groups.Proxies {
		if group.Type == "Selector" && group.Now != "" {
			selected[name] = group.Now
		}
	}
	if err := s.stopLocked(); err != nil {
		return fmt.Errorf("恢复网络失败，未重启：%w", err)
	}
	if err := s.startLocked(string(updated), port, bypass, lan, options, selected); err != nil {
		return fmt.Errorf("内核重启失败，请检查错误并重新启动：%w", err)
	}
	return nil
}

// Called with mu held, after the controller is ready and before system takeover.
func (s *Service) restoreSelections(selected map[string]string) error {
	for group, node := range selected {
		if err := s.request("PUT", "/proxies/"+url.PathEscape(group), map[string]string{"name": node}); err != nil {
			return fmt.Errorf("恢复策略组 %s 的节点失败：%w", group, err)
		}
	}
	return nil
}

// snapshotSelections records current Selector choices. Called with mu held;
// on failure the previous snapshot is kept.
func (s *Service) snapshotSelections() {
	var groups struct {
		Proxies map[string]Proxy `json:"proxies"`
	}
	if err := s.get("/proxies", &groups); err != nil {
		return
	}
	saved := make(map[string]string)
	for name, group := range groups.Proxies {
		if group.Type == "Selector" && group.Now != "" {
			saved[name] = group.Now
		}
	}
	s.savedSelections = saved
}

// restoreSavedSelections is best effort: groups or nodes missing from the new
// profile are skipped. Called with mu held, before system takeover.
func (s *Service) restoreSavedSelections() {
	for group, node := range s.savedSelections {
		_ = s.request("PUT", "/proxies/"+url.PathEscape(group), map[string]string{"name": node})
	}
}
