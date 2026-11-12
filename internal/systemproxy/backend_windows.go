//go:build windows

package systemproxy

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

const winINETServiceID = "wininet"

// WinINETAccess reads and writes the current user's Internet Settings key.
type WinINETAccess interface {
	Read() (winINETSnapshot, error)
	Write(winINETSnapshot) error
	Notify() error
}

type registryWinINET struct{}

func (registryWinINET) key() (registry.Key, error) {
	return registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.SET_VALUE|registry.QUERY_VALUE)
}

func (registryWinINET) Read() (winINETSnapshot, error) {
	k, err := registryWinINET{}.key()
	if err != nil {
		return winINETSnapshot{}, fmt.Errorf("读取 Internet 设置失败：%w", err)
	}
	defer k.Close()
	var s winINETSnapshot
	if v, _, err := k.GetIntegerValue("ProxyEnable"); err == nil {
		s.ProxyEnable = v != 0
	}
	if v, _, err := k.GetStringValue("ProxyServer"); err == nil {
		s.ProxyServer = v
	}
	if v, _, err := k.GetStringValue("ProxyOverride"); err == nil {
		s.ProxyOverride = v
	}
	if v, _, err := k.GetStringValue("AutoConfigURL"); err == nil {
		s.AutoConfigURL = v
	}
	if v, _, err := k.GetIntegerValue("AutoDetect"); err == nil {
		s.AutoDiscovery = v != 0
	}
	return s, nil
}

func (registryWinINET) Write(s winINETSnapshot) error {
	k, err := registryWinINET{}.key()
	if err != nil {
		return fmt.Errorf("写入 Internet 设置失败：%w", err)
	}
	defer k.Close()
	enable := uint32(0)
	if s.ProxyEnable {
		enable = 1
	}
	if err := k.SetDWordValue("ProxyEnable", enable); err != nil {
		return err
	}
	if s.ProxyServer != "" {
		if err := k.SetStringValue("ProxyServer", s.ProxyServer); err != nil {
			return err
		}
	}
	if err := k.SetStringValue("ProxyOverride", s.ProxyOverride); err != nil {
		return err
	}
	if s.AutoConfigURL != "" {
		if err := k.SetStringValue("AutoConfigURL", s.AutoConfigURL); err != nil {
			return err
		}
	} else {
		_ = k.DeleteValue("AutoConfigURL")
	}
	detect := uint32(0)
	if s.AutoDiscovery {
		detect = 1
	}
	if err := k.SetDWordValue("AutoDetect", detect); err != nil {
		return err
	}
	return nil
}

func (registryWinINET) Notify() error { return notifyWinINET() }

// WindowsBackend applies Internet Settings for the current user (WinINET).
type WindowsBackend struct {
	Access WinINETAccess
}

func (b WindowsBackend) access() WinINETAccess {
	if b.Access != nil {
		return b.Access
	}
	return registryWinINET{}
}

func (b WindowsBackend) Snapshot() ([]Service, error) {
	raw, err := b.access().Read()
	if err != nil {
		return nil, err
	}
	return []Service{{
		ID:       winINETServiceID,
		Name:     "Internet 设置",
		Settings: settingsFromWinINET(raw),
	}}, nil
}

func (b WindowsBackend) CompareAndSwap(changes []Change) ([]string, error) {
	return b.compareAndSwap(changes, false)
}

func (b WindowsBackend) CompareAndSwapPartial(changes []Change) ([]string, error) {
	return b.compareAndSwap(changes, true)
}

func (b WindowsBackend) compareAndSwap(changes []Change, partial bool) ([]string, error) {
	services, err := b.Snapshot()
	if err != nil {
		return nil, err
	}
	current, err := b.access().Read()
	if err != nil {
		return nil, err
	}
	conflicts, updated, applied := applyChanges(services, changes)
	if len(updated) != 1 || updated[0].ID != winINETServiceID {
		return nil, fmt.Errorf("Internet 设置状态无效")
	}
	if len(conflicts) > 0 && !partial {
		return conflicts, nil
	}
	if len(applied) == 0 {
		return conflicts, nil
	}
	next := applyWinINETChanges(current, updated[0].Settings, applied)
	if err = b.access().Write(next); err != nil {
		return nil, err
	}
	if err = b.access().Notify(); err != nil {
		return nil, err
	}
	return conflicts, nil
}
