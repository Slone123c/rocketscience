//go:build linux

package systemproxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// GSettingsRunner runs gsettings without a shell.
type GSettingsRunner interface {
	Get(schema, key string) (string, error)
	Set(schema, key, value string) error
}

type execGSettings struct {
	bin string
}

func (g execGSettings) Get(schema, key string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, g.bin, "get", schema, key)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (g execGSettings) Set(schema, key, value string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, g.bin, "set", schema, key, value)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

func findGSettings() (string, error) {
	if bin := strings.TrimSpace(os.Getenv("ROCKETSCIENCE_GSETTINGS")); bin != "" {
		if _, err := os.Stat(bin); err == nil {
			return bin, nil
		}
		return "", fmt.Errorf("未找到 gsettings：%s", bin)
	}
	path, err := exec.LookPath("gsettings")
	if err != nil {
		return "", errors.New("未检测到受支持的 Linux 桌面代理 API（需要 GNOME gsettings）")
	}
	return path, nil
}

func gnomeDesktopSelected() bool {
	desktop := strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP"))
	if strings.Contains(desktop, "gnome") || strings.Contains(desktop, "unity") {
		return true
	}
	if os.Getenv("DESKTOP_SESSION") == "gnome" {
		return true
	}
	// Allow explicit opt-in when headless tests export ROCKETSCIENCE_GSETTINGS.
	return strings.TrimSpace(os.Getenv("ROCKETSCIENCE_GSETTINGS")) != ""
}

type gnomeStore struct {
	runner GSettingsRunner
}

func (g gnomeStore) read() (gnomeSnapshot, error) {
	var s gnomeSnapshot
	mode, err := g.runner.Get("org.gnome.system.proxy", "mode")
	if err != nil {
		return s, err
	}
	s.Mode, err = parseGsettingsValue(mode)
	if err != nil {
		return s, err
	}
	httpHost, err := g.runner.Get("org.gnome.system.proxy.http", "host")
	if err != nil {
		return s, err
	}
	s.HTTPHost, err = parseGsettingsValue(httpHost)
	if err != nil {
		return s, err
	}
	portRaw, err := g.runner.Get("org.gnome.system.proxy.http", "port")
	if err != nil {
		return s, err
	}
	if portRaw != "" {
		s.HTTPPort = parseGsettingsUInt(portRaw)
	}
	httpsHost, err := g.runner.Get("org.gnome.system.proxy.https", "host")
	if err != nil {
		return s, err
	}
	s.HTTPSHost, err = parseGsettingsValue(httpsHost)
	if err != nil {
		return s, err
	}
	portRaw, err = g.runner.Get("org.gnome.system.proxy.https", "port")
	if err != nil {
		return s, err
	}
	if portRaw != "" {
		s.HTTPSPort = parseGsettingsUInt(portRaw)
	}
	socksHost, err := g.runner.Get("org.gnome.system.proxy.socks", "host")
	if err != nil {
		return s, err
	}
	s.SOCKSHost, err = parseGsettingsValue(socksHost)
	if err != nil {
		return s, err
	}
	portRaw, err = g.runner.Get("org.gnome.system.proxy.socks", "port")
	if err != nil {
		return s, err
	}
	if portRaw != "" {
		s.SOCKSPort = parseGsettingsUInt(portRaw)
	}
	ignore, err := g.runner.Get("org.gnome.system.proxy", "ignore-hosts")
	if err != nil {
		return s, err
	}
	s.IgnoreHosts, err = parseGsettingsStringArray(ignore)
	if err != nil {
		return s, err
	}
	autoURL, _ := g.runner.Get("org.gnome.system.proxy", "autoconfig-url")
	s.AutoURL, _ = parseGsettingsValue(autoURL)
	return s, nil
}

func (g gnomeStore) write(s gnomeSnapshot) error {
	before, err := g.read()
	if err != nil {
		return err
	}
	if err := g.writeDirect(s); err != nil {
		_ = g.writeDirect(before)
		return err
	}
	return nil
}

func (g gnomeStore) writeDirect(s gnomeSnapshot) error {
	apply := func() error {
		if err := g.runner.Set("org.gnome.system.proxy", "mode", formatGsettingsString(s.Mode)); err != nil {
			return err
		}
		switch s.Mode {
		case "auto":
			if err := g.runner.Set("org.gnome.system.proxy", "autoconfig-url", formatGsettingsString(s.AutoURL)); err != nil {
				return err
			}
		default:
			if err := g.runner.Set("org.gnome.system.proxy", "autoconfig-url", formatGsettingsString("")); err != nil {
				return err
			}
		}
		if s.Mode != "manual" {
			return nil
		}
		for _, item := range []struct {
			schema string
			host   string
			port   int
		}{
			{"org.gnome.system.proxy.http", s.HTTPHost, s.HTTPPort},
			{"org.gnome.system.proxy.https", s.HTTPSHost, s.HTTPSPort},
			{"org.gnome.system.proxy.socks", s.SOCKSHost, s.SOCKSPort},
		} {
			if err := g.runner.Set(item.schema, "host", formatGsettingsString(item.host)); err != nil {
				return err
			}
			if err := g.runner.Set(item.schema, "port", formatGsettingsUInt(item.port)); err != nil {
				return err
			}
		}
		if err := g.runner.Set("org.gnome.system.proxy", "ignore-hosts", formatGsettingsStringArray(s.IgnoreHosts)); err != nil {
			return err
		}
		return nil
	}
	return apply()
}

func strconvAtoi(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	var n int
	_, err := fmt.Sscanf(value, "%d", &n)
	return n, err
}

// LinuxBackend applies GNOME gsettings proxy preferences when available.
type LinuxBackend struct {
	Runner GSettingsRunner
}

func (b LinuxBackend) store() (gnomeStore, error) {
	if !gnomeDesktopSelected() {
		return gnomeStore{}, errors.New("未检测到受支持的 Linux 桌面环境（当前实现支持 GNOME）")
	}
	if b.Runner != nil {
		return gnomeStore{runner: b.Runner}, nil
	}
	bin, err := findGSettings()
	if err != nil {
		return gnomeStore{}, err
	}
	return gnomeStore{runner: execGSettings{bin: bin}}, nil
}

func (b LinuxBackend) Snapshot() ([]Service, error) {
	store, err := b.store()
	if err != nil {
		return nil, err
	}
	raw, err := store.read()
	if err != nil {
		return nil, fmt.Errorf("读取桌面代理设置失败：%w", err)
	}
	return []Service{{
		ID:       gnomeServiceID,
		Name:     "GNOME 系统代理",
		Settings: settingsFromGnome(raw),
	}}, nil
}

func (b LinuxBackend) CompareAndSwap(changes []Change) ([]string, error) {
	return b.compareAndSwap(changes, false)
}

func (b LinuxBackend) CompareAndSwapPartial(changes []Change) ([]string, error) {
	return b.compareAndSwap(changes, true)
}

func (b LinuxBackend) compareAndSwap(changes []Change, partial bool) ([]string, error) {
	store, err := b.store()
	if err != nil {
		return nil, err
	}
	services, err := b.Snapshot()
	if err != nil {
		return nil, err
	}
	conflicts, updated, applied := applyChanges(services, changes)
	if len(updated) != 1 || updated[0].ID != gnomeServiceID {
		return nil, fmt.Errorf("桌面代理状态无效")
	}
	if len(conflicts) > 0 && !partial {
		return conflicts, nil
	}
	if len(applied) == 0 {
		return conflicts, nil
	}
	current, err := store.read()
	if err != nil {
		return nil, err
	}
	next := applyGnomeChanges(current, updated[0].Settings, applied)
	if err = store.write(next); err != nil {
		return nil, err
	}
	return conflicts, nil
}
