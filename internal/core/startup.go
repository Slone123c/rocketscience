package core

import (
	"errors"
	"fmt"
)

func (p *Profiles) AutoConnect() (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	return d.AutoConnect, err
}
func (p *Profiles) SetAutoConnect(value bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return err
	}
	d.AutoConnect = value
	return p.write(d)
}

func (p *Profiles) HideDockWhenClosed() (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	return d.HideDockWhenClosed, err
}
func (p *Profiles) SetHideDockWhenClosed(value bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return err
	}
	d.HideDockWhenClosed = value
	return p.write(d)
}

// Initialize applies the persisted takeover choices before any automatic start.
// It is called once by the desktop host before controls become interactive.
func (p *Profiles) Initialize() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.service == nil {
		return errors.New("内核服务不可用")
	}
	d, changed, err := p.loadWithNormalization()
	if err != nil {
		return err
	}
	if changed {
		if err = p.write(d); err != nil {
			return fmt.Errorf("无法更新不支持环境的接管设置：%w", err)
		}
	}
	if err = p.service.SetSystemProxy(d.SystemProxy); err != nil {
		return fmt.Errorf("无法加载系统代理设置：%w", err)
	}
	if err = p.service.SetTun(d.Tun); err != nil {
		return fmt.Errorf("无法加载 TUN 设置：%w", err)
	}
	if !d.AutoConnect {
		return nil
	}
	if takeoverRequiredBeforeStart() && !d.SystemProxy && !d.Tun {
		return errors.New("自动连接未启动：请先选择一种接管方式")
	}
	if err = p.startActiveLocked(d); err != nil {
		return fmt.Errorf("自动连接未启动：%w", err)
	}
	return nil
}
