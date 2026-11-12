package core

import (
	"errors"
	"rocketscience/internal/systemproxy"
	"strings"
)

type proxyController interface {
	ApplyWithBypass(int, *[]string) error
	Restore() ([]string, error)
	Pending() (bool, error)
}

// SetSystemProxy selects whether the core takes over traffic while running.
func (s *Service) SetSystemProxy(enabled bool) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if enabled && s.proxy == nil {
		return errors.New("系统代理在当前平台不可用")
	}
	if enabled && !systemproxy.AutomaticAvailable() {
		return errors.New(systemproxy.ManualFallbackNote())
	}
	if s.useSystemProxy == enabled {
		return nil
	}
	// Restore and stop only when this is the last active takeover.
	if !enabled && !s.useTun && s.cmd != nil {
		s.mu.Unlock()
		err := s.stopLocked()
		s.mu.Lock()
		if err != nil {
			return err
		}
		s.useSystemProxy = false
		return nil
	}
	if s.cmd != nil {
		if enabled {
			if err := s.proxy.ApplyWithBypass(s.mixedPort, s.proxyBypass); err != nil {
				var recoverErr error
				if errors.Is(err, systemproxy.ErrRecoveryNeeded) {
					_, recoverErr = s.proxy.Restore()
				}
				if recoverErr != nil {
					s.ownsProxy = true
					s.proxyWarning = "系统代理恢复失败，请先恢复网络：" + recoverErr.Error()
				}
				return err
			}
			s.ownsProxy = true
			s.proxyWarning = ""
		} else if s.proxy != nil && s.ownsProxy {
			conflicts, err := s.proxy.Restore()
			if err != nil {
				return err
			}
			s.recordConflicts(conflicts)
			s.ownsProxy = false
		}
	}
	s.useSystemProxy = enabled
	return nil
}
func (s *Service) recordConflicts(conflicts []string) {
	if len(conflicts) > 0 {
		s.proxyWarning = "部分网络服务已被其他程序修改，未覆盖其设置：" + strings.Join(conflicts, ", ")
	} else {
		s.proxyWarning = ""
	}
}
func (s *Service) recoverAfterExit() {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.proxy == nil || !s.ownsProxy {
		return
	}
	conflicts, err := s.proxy.Restore()
	if err != nil {
		s.proxyWarning = "内核退出后恢复系统代理失败：" + err.Error()
		return
	}
	s.recordConflicts(conflicts)
	s.ownsProxy = false
}

// RecoverNetwork restores any recorded takeover before stopping the core.
func (s *Service) RecoverNetwork() error {
	if err := s.Stop(); err != nil {
		return err
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.proxy != nil {
		conflicts, err := s.proxy.Restore()
		if err != nil {
			return err
		}
		s.recordConflicts(conflicts)
		s.ownsProxy = false
	}
	return nil
}
