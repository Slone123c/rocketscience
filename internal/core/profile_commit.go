package core

import (
	"errors"
	"fmt"
)

// commitConfiguration serializes live reload, persistence and rollback against
// starts, stops, mode changes and node selection. persist never calls the service.
func (s *Service) commitConfiguration(content string, persist func() error) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil {
		return persist()
	}
	if s.stopping {
		return errors.New("内核正在停止，请稍后保存或切换配置")
	}
	var current struct {
		Mode string `json:"mode"`
	}
	var groups struct {
		Proxies map[string]Proxy `json:"proxies"`
	}
	if err := s.get("/configs", &current); err != nil {
		return err
	}
	if err := s.get("/proxies", &groups); err != nil {
		return err
	}
	selected := map[string]string{}
	for name, group := range groups.Proxies {
		if group.Type == "Selector" && group.Now != "" {
			selected[name] = group.Now
		}
	}
	next, err := runtimeConfigWithOptions(content, s.mixedPort, s.port, s.secret, current.Mode, s.lan, s.useTun, s.coreOptions)
	if err != nil {
		return err
	}
	previous, err := runtimeConfigWithOptions(s.currentContent, s.mixedPort, s.port, s.secret, current.Mode, s.lan, s.useTun, s.coreOptions)
	if err != nil {
		return err
	}
	if string(next) == string(previous) {
		if err := persist(); err != nil {
			return fmt.Errorf("保存配置失败：%w", err)
		}
		s.currentContent = content
		return nil
	}
	apply := func(payload []byte) error {
		if err := s.markGeoAuto(payload); err != nil {
			return err
		}
		return s.request("PUT", "/configs?force=true", map[string]string{"payload": string(payload)})
	}
	rollback := func(cause error) error {
		if err := apply(previous); err != nil {
			s.lastError = "配置操作失败且旧配置恢复失败，请停止代理并恢复网络"
			return errors.Join(cause, fmt.Errorf("恢复旧运行配置失败：%w", err))
		}
		if err := s.restoreSelections(selected); err != nil {
			s.lastError = "旧配置已恢复，但节点选择恢复失败"
			return errors.Join(cause, err)
		}
		return cause
	}
	if err := apply(next); err != nil {
		return rollback(err)
	}
	// Keep choices only when both the group and its selected node still exist.
	var available struct {
		Proxies map[string]Proxy `json:"proxies"`
	}
	if err := s.get("/proxies", &available); err != nil {
		return rollback(err)
	}
	retained := map[string]string{}
	for name, node := range selected {
		group := available.Proxies[name]
		if group.Type != "Selector" {
			continue
		}
		for _, member := range group.All {
			if member == node {
				retained[name] = node
				break
			}
		}
	}
	if err := s.restoreSelections(retained); err != nil {
		return rollback(err)
	}
	if err := persist(); err != nil {
		return rollback(fmt.Errorf("保存配置失败：%w", err))
	}
	s.currentContent = content
	s.savedSelections = retained
	return nil
}

// Called with Profiles.mu held. Overrides are rebuilt for the chosen profile.
func (p *Profiles) commitProfile(d profileData, content string) error {
	merged, _, _, _, err := mergeOverrides(content, d.Overrides)
	if err != nil {
		return err
	}
	if err := validateProfile(merged); err != nil {
		return err
	}
	persist := func() error { return p.write(d) }
	if p.service != nil {
		err = p.service.commitConfiguration(merged, persist)
	} else {
		err = persist()
	}
	if err == nil {
		p.draft = content
	}
	return err
}
