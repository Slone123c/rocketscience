package core

import (
	"errors"
	"fmt"
	"runtime"
	"runtime/debug"

	C "github.com/metacubex/mihomo/constant"
	"gopkg.in/yaml.v3"
)

type KernelInfo struct {
	Available bool   `json:"available"`
	Path      string `json:"path"`
	Version   string `json:"version"`
	Platform  string `json:"platform"`
}

func mihomoVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range info.Deps {
			if dep.Path == "github.com/metacubex/mihomo" && dep.Version != "" {
				return dep.Version
			}
		}
	}
	return "v1.19.33"
}

func init() {
	C.Version = mihomoVersion()
}

// KernelInfo works while stopped and never starts a controller or takes over traffic.
func (s *Service) KernelInfo() (KernelInfo, error) {
	return KernelInfo{
		Available: true,
		Path:      "embedded",
		Version:   fmt.Sprintf("Mihomo Meta %s %s %s with %s", mihomoVersion(), runtime.GOOS, runtime.GOARCH, runtime.Version()),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}, nil
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
	mode := currentInProcessMode()
	proxies, err := inProcessProxiesMap()
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("读取当前节点失败，未重启：%w", err)
	}
	content, port, bypass, lan, options := s.currentContent, s.mixedPort, cloneBypass(s.proxyBypass), s.lan, s.coreOptions
	s.mu.Unlock()
	document, err := unmarshalConfigMap([]byte(content))
	if err != nil {
		return err
	}
	if document == nil {
		return errors.New("配置不能为空")
	}
	document["mode"] = mode
	updated, err := yaml.Marshal(document)
	if err != nil {
		return err
	}
	selected := make(map[string]string)
	for name, group := range proxies {
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
		if err := selectInProcessProxy(group, node); err != nil {
			return fmt.Errorf("恢复策略组 %s 的节点失败：%w", group, err)
		}
	}
	return nil
}

// snapshotSelections records current Selector choices. Called with mu held;
// on failure the previous snapshot is kept.
func (s *Service) snapshotSelections() {
	proxies, err := inProcessProxiesMap()
	if err != nil {
		return
	}
	saved := make(map[string]string)
	for name, group := range proxies {
		if group.Type == "Selector" && group.Now != "" {
			saved[name] = group.Now
		}
	}
	s.savedSelections = saved
	s.persistSelectionsLocked()
}

// restoreSavedSelections is best effort: groups or nodes missing from the new
// profile are skipped. Called with mu held, before system takeover.
func (s *Service) restoreSavedSelections() {
	for group, node := range s.savedSelections {
		_ = selectInProcessProxy(group, node)
	}
}
