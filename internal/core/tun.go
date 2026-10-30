package core

import (
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

// SetTun changes the selected takeover. When running, it restarts the core so
// authorization applies only to TUN sessions; an unsuccessful restart rolls back.
func (s *Service) SetTun(enabled bool) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	s.mu.Lock()
	if s.useTun == enabled {
		s.mu.Unlock()
		return nil
	}
	if s.cmd == nil {
		s.useTun = enabled
		s.mu.Unlock()
		return nil
	}
	previous := s.useTun
	systemProxy := s.useSystemProxy
	content, port, bypass, lan, options := s.currentContent, s.mixedPort, cloneBypass(s.proxyBypass), s.lan, s.coreOptions
	mode := currentInProcessMode()
	s.mu.Unlock()
	document, err := unmarshalConfigMap([]byte(content))
	if err != nil {
		return err
	}
	if document == nil {
		return errors.New("配置不能为空")
	}
	if mode != "" {
		document["mode"] = mode
	}
	updated, err := yaml.Marshal(document)
	if err != nil {
		return err
	}
	content = string(updated)
	if err = s.stopLocked(); err != nil {
		return err
	}
	s.mu.Lock()
	s.useTun = enabled
	s.mu.Unlock()
	if !enabled && !systemProxy {
		return nil
	}
	err = s.startLocked(content, port, bypass, lan, options)
	if err == nil {
		return nil
	}
	s.mu.Lock()
	stillRunning := s.cmd != nil
	if !stillRunning {
		s.useTun = previous
	}
	s.mu.Unlock()
	if stillRunning {
		return fmt.Errorf("切换失败，内核仍在停止，请先恢复网络：%w", err)
	}
	if rollbackErr := s.startLocked(content, port, bypass, lan, options); rollbackErr != nil {
		return errors.Join(fmt.Errorf("切换 TUN 失败：%w", err), fmt.Errorf("恢复先前运行状态失败：%w", rollbackErr))
	}
	return fmt.Errorf("切换 TUN 失败，已恢复先前运行状态：%w", err)
}

func (p *Profiles) TunEnabled() (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	return d.Tun, err
}

// ChangeTun applies the selection before committing it, rolling back on save failure.
func (p *Profiles) ChangeTun(enabled bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.service == nil {
		return errors.New("内核服务不可用")
	}
	d, err := p.load()
	if err != nil {
		return err
	}
	p.service.mu.Lock()
	previous := p.service.useTun
	previousOptions := p.service.coreOptions
	p.service.mu.Unlock()
	if err = p.service.SetTun(enabled); err != nil {
		return err
	}
	d.Tun = enabled
	if enabled {
		options := enabledTunDNS(coreOptionsValue(d.CoreOptions))
		d.CoreOptions = &options
	}
	if err = p.write(d); err != nil {
		if rollback := p.service.SetTun(previous); rollback != nil {
			return errors.Join(fmt.Errorf("保存 TUN 设置失败：%w", err), fmt.Errorf("恢复先前状态失败：%w", rollback))
		}
		if _, rollback := p.service.applyCoreOptions(previousOptions); rollback != nil {
			return errors.Join(fmt.Errorf("保存 TUN 设置失败：%w", err), fmt.Errorf("恢复先前 DNS 状态失败：%w", rollback))
		}
		return fmt.Errorf("保存 TUN 设置失败：%w", err)
	}
	return nil
}
