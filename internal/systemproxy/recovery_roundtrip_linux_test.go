//go:build linux

package systemproxy

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxBackendRecoveryRoundTrip_autoAndManual(t *testing.T) {
	t.Setenv("XDG_CURRENT_DESKTOP", "GNOME")
	journal := filepath.Join(t.TempDir(), "proxy-recovery.json")

	t.Run("PAC auto", func(t *testing.T) {
		runner := &mapGSettings{data: map[string]string{
			"org.gnome.system.proxy\x00mode":           `'auto'`,
			"org.gnome.system.proxy\x00autoconfig-url": `'http://pac.corp/proxy.pac'`,
			"org.gnome.system.proxy\x00ignore-hosts":  `@as []`,
		}}
		wantMode, _ := runner.Get("org.gnome.system.proxy", "mode")
		wantURL, _ := runner.Get("org.gnome.system.proxy", "autoconfig-url")
		backend := LinuxBackend{Runner: runner}
		if err := New(backend, journal).Apply(7890); err != nil {
			t.Fatal(err)
		}
		mode, _ := runner.Get("org.gnome.system.proxy", "mode")
		assertGsettingsString(t, mode, `'manual'`)
		if conflicts, err := New(LinuxBackend{Runner: runner}, journal).Restore(); err != nil {
			t.Fatalf("restore: %v", err)
		} else if len(conflicts) > 0 {
			t.Fatalf("conflicts=%v", conflicts)
		}
		mode, _ = runner.Get("org.gnome.system.proxy", "mode")
		url, _ := runner.Get("org.gnome.system.proxy", "autoconfig-url")
		assertGsettingsString(t, mode, wantMode)
		assertGsettingsString(t, url, wantURL)
		services, err := backend.Snapshot()
		if err != nil {
			t.Fatal(err)
		}
		if !parseBool(services[0].Settings["ProxyAutoConfigEnable"]) {
			t.Fatal("fresh snapshot lost PAC enable")
		}
		if parseString(services[0].Settings[gnomeKeyAutoURL]) != "http://pac.corp/proxy.pac" {
			t.Fatalf("fresh auto URL=%q", services[0].Settings[gnomeKeyAutoURL])
		}
	})

	t.Run("manual disabled endpoints", func(t *testing.T) {
		runner := &mapGSettings{data: map[string]string{
			"org.gnome.system.proxy\x00mode":           `'manual'`,
			"org.gnome.system.proxy.http\x00host":        `'corp.example'`,
			"org.gnome.system.proxy.http\x00port":        `uint32 8080`,
			"org.gnome.system.proxy.https\x00host":       `'secure.corp'`,
			"org.gnome.system.proxy.https\x00port":       `uint32 8443`,
			"org.gnome.system.proxy.socks\x00host":       `'socks.corp'`,
			"org.gnome.system.proxy.socks\x00port":       `uint32 1080`,
			"org.gnome.system.proxy\x00ignore-hosts":     `['corp.lan']`,
			"org.gnome.system.proxy\x00autoconfig-url":   `''`,
		}}
		backend := LinuxBackend{Runner: runner}
		j := filepath.Join(t.TempDir(), "proxy-recovery.json")
		if err := New(backend, j).Apply(7890); err != nil {
			t.Fatal(err)
		}
		if _, err := New(LinuxBackend{Runner: runner}, j).Restore(); err != nil {
			t.Fatal(err)
		}
		mode, _ := runner.Get("org.gnome.system.proxy", "mode")
		assertGsettingsString(t, mode, `'manual'`)
		host, _ := runner.Get("org.gnome.system.proxy.http", "host")
		assertGsettingsString(t, host, `'corp.example'`)
		ignore, _ := runner.Get("org.gnome.system.proxy", "ignore-hosts")
		assertGsettingsStringArray(t, ignore, `['corp.lan']`)
	})

	t.Run("external bypass edit partial restore", func(t *testing.T) {
		originalIgnore := `['corp.lan']`
		runner := &mapGSettings{data: map[string]string{
			"org.gnome.system.proxy\x00mode":         `'manual'`,
			"org.gnome.system.proxy.http\x00host":      `'corp.example'`,
			"org.gnome.system.proxy.http\x00port":      `uint32 8080`,
			"org.gnome.system.proxy.https\x00host":     `'secure.corp'`,
			"org.gnome.system.proxy.https\x00port":     `uint32 8443`,
			"org.gnome.system.proxy.socks\x00host":    `'socks.corp'`,
			"org.gnome.system.proxy.socks\x00port":    `uint32 1080`,
			"org.gnome.system.proxy\x00ignore-hosts":   originalIgnore,
			"org.gnome.system.proxy\x00autoconfig-url": `''`,
		}}
		j := filepath.Join(t.TempDir(), "proxy-recovery.json")
		backend := LinuxBackend{Runner: runner}
		if err := New(backend, j).Apply(7890); err != nil {
			t.Fatal(err)
		}
		runner.data["org.gnome.system.proxy\x00ignore-hosts"] = `['user-edited']`
		conflicts, err := New(LinuxBackend{Runner: runner}, j).Restore()
		if err != nil {
			t.Fatal(err)
		}
		if len(conflicts) == 0 {
			t.Fatal("expected ignore-hosts group conflict")
		}
		mode, _ := runner.Get("org.gnome.system.proxy", "mode")
		assertGsettingsString(t, mode, `'manual'`)
		host, _ := runner.Get("org.gnome.system.proxy.http", "host")
		assertGsettingsString(t, host, `'corp.example'`)
		port, _ := runner.Get("org.gnome.system.proxy.http", "port")
		if port != `8080` {
			t.Fatalf("http port=%q", port)
		}
		ignore, _ := runner.Get("org.gnome.system.proxy", "ignore-hosts")
		assertGsettingsStringArray(t, ignore, `['user-edited']`)
		if _, err := os.Stat(j); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("journal removed after successful partial restore")
		}
		if err := New(backend, j).Apply(7890); err != nil {
			t.Fatal("fresh takeover after partial restore")
		}
	})

	t.Run("custom bypass apply and restore", func(t *testing.T) {
		originalIgnore := `['corp.lan']`
		runner := &mapGSettings{data: map[string]string{
			"org.gnome.system.proxy\x00mode":           `'manual'`,
			"org.gnome.system.proxy.http\x00host":      `'127.0.0.1'`,
			"org.gnome.system.proxy.http\x00port":      `uint32 1`,
			"org.gnome.system.proxy.https\x00host":     `'127.0.0.1'`,
			"org.gnome.system.proxy.https\x00port":     `uint32 1`,
			"org.gnome.system.proxy.socks\x00host":     `'127.0.0.1'`,
			"org.gnome.system.proxy.socks\x00port":     `uint32 1`,
			"org.gnome.system.proxy\x00ignore-hosts":   originalIgnore,
			"org.gnome.system.proxy\x00autoconfig-url": `''`,
		}}
		j := filepath.Join(t.TempDir(), "proxy-recovery.json")
		backend := LinuxBackend{Runner: runner}
		bypass := []string{"corp.lan", "extra.bypass.test"}
		if err := New(backend, j).ApplyWithBypass(7890, &bypass); err != nil {
			t.Fatal(err)
		}
		ignore, _ := runner.Get("org.gnome.system.proxy", "ignore-hosts")
		if !strings.Contains(ignore, "extra.bypass.test") || !strings.Contains(ignore, "localhost") {
			t.Fatalf("applied ignore-hosts=%q", ignore)
		}
		if _, err := New(LinuxBackend{Runner: runner}, j).Restore(); err != nil {
			t.Fatal(err)
		}
		ignore, _ = runner.Get("org.gnome.system.proxy", "ignore-hosts")
		assertGsettingsStringArray(t, ignore, originalIgnore)
	})

	t.Run("none", func(t *testing.T) {
		runner := &mapGSettings{data: map[string]string{
			"org.gnome.system.proxy\x00mode": `'none'`,
		}}
		j := filepath.Join(t.TempDir(), "proxy-recovery.json")
		if err := New(LinuxBackend{Runner: runner}, j).Apply(7890); err != nil {
			t.Fatal(err)
		}
		if _, err := New(LinuxBackend{Runner: runner}, j).Restore(); err != nil {
			t.Fatal(err)
		}
		mode, _ := runner.Get("org.gnome.system.proxy", "mode")
		assertGsettingsString(t, mode, `'none'`)
	})
}
