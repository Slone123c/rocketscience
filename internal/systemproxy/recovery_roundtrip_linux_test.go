//go:build linux

package systemproxy

import (
	"path/filepath"
	"testing"
)

func TestLinuxBackendRecoveryRoundTrip_autoAndManual(t *testing.T) {
	t.Setenv("XDG_CURRENT_DESKTOP", "GNOME")
	journal := filepath.Join(t.TempDir(), "proxy-recovery.json")

	t.Run("PAC auto", func(t *testing.T) {
		runner := &mapGSettings{data: map[string]string{
			"org.gnome.system.proxy\x00mode":            `'auto'`,
			"org.gnome.system.proxy\x00autoconfig-url":  `'http://pac.corp/proxy.pac'`,
			"org.gnome.system.proxy\x00ignore-hosts":   `@as []`,
		}}
		wantMode, _ := runner.Get("org.gnome.system.proxy", "mode")
		wantURL, _ := runner.Get("org.gnome.system.proxy", "autoconfig-url")
		backend := LinuxBackend{Runner: runner}
		if err := New(backend, journal).Apply(7890); err != nil {
			t.Fatal(err)
		}
		mode, _ := runner.Get("org.gnome.system.proxy", "mode")
		if mode != `'manual'` {
			t.Fatalf("after apply mode=%q", mode)
		}
		if conflicts, err := New(LinuxBackend{Runner: runner}, journal).Restore(); err != nil {
			t.Fatalf("restore: %v", err)
		} else if len(conflicts) > 0 {
			t.Fatalf("conflicts=%v", conflicts)
		}
		mode, _ = runner.Get("org.gnome.system.proxy", "mode")
		url, _ := runner.Get("org.gnome.system.proxy", "autoconfig-url")
		if mode != wantMode || url != wantURL {
			t.Fatalf("restored mode=%q url=%q want %q %q", mode, url, wantMode, wantURL)
		}
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
			"org.gnome.system.proxy\x00mode":                  `'manual'`,
			"org.gnome.system.proxy.http\x00host":             `'corp.example'`,
			"org.gnome.system.proxy.http\x00port":             `uint32 8080`,
			"org.gnome.system.proxy.https\x00host":            `'secure.corp'`,
			"org.gnome.system.proxy.https\x00port":            `uint32 8443`,
			"org.gnome.system.proxy.socks\x00host":            `'socks.corp'`,
			"org.gnome.system.proxy.socks\x00port":            `uint32 1080`,
			"org.gnome.system.proxy\x00ignore-hosts":          `['corp.lan']`,
			"org.gnome.system.proxy\x00autoconfig-url":        `''`,
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
		if mode != `'manual'` {
			t.Fatalf("mode=%q", mode)
		}
		host, _ := runner.Get("org.gnome.system.proxy.http", "host")
		if host != `'corp.example'` {
			t.Fatalf("http host=%q", host)
		}
		ignore, _ := runner.Get("org.gnome.system.proxy", "ignore-hosts")
		if ignore != `['corp.lan']` {
			t.Fatalf("ignore-hosts=%q", ignore)
		}
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
		if mode != `'none'` {
			t.Fatalf("mode=%q", mode)
		}
	})
}
