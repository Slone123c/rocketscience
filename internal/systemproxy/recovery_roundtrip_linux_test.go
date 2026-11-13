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

	t.Run("PAC auto", func(t *testing.T) {
		journal := filepath.Join(t.TempDir(), "proxy-recovery.json")
		runner := &mapGSettings{data: map[string]string{
			"org.gnome.system.proxy\x00mode":           `'auto'`,
			"org.gnome.system.proxy\x00autoconfig-url": `'http://pac.corp/proxy.pac'`,
			"org.gnome.system.proxy\x00ignore-hosts":   `@as []`,
		}}
		wantMode, _ := runner.Get("org.gnome.system.proxy", "mode")
		wantURL, _ := runner.Get("org.gnome.system.proxy", "autoconfig-url")
		backend := LinuxBackend{Runner: runner}
		if err := New(backend, journal).Apply(7890); err != nil {
			t.Fatal(err)
		}
		services, err := backend.Snapshot()
		if err != nil {
			t.Fatal(err)
		}
		if parseBool(services[0].Settings["ProxyAutoConfigEnable"]) {
			t.Fatalf("after apply ProxyAutoConfigEnable should be off when mode manual: %+v", services[0].Settings)
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
		services, err = backend.Snapshot()
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
			"org.gnome.system.proxy.http\x00host":      `'corp.example'`,
			"org.gnome.system.proxy.http\x00port":      `uint32 8080`,
			"org.gnome.system.proxy.https\x00host":     `'secure.corp'`,
			"org.gnome.system.proxy.https\x00port":     `uint32 8443`,
			"org.gnome.system.proxy.socks\x00host":     `'socks.corp'`,
			"org.gnome.system.proxy.socks\x00port":     `uint32 1080`,
			"org.gnome.system.proxy\x00ignore-hosts":   `['corp.lan']`,
			"org.gnome.system.proxy\x00autoconfig-url": `''`,
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
			"org.gnome.system.proxy\x00mode":           `'manual'`,
			"org.gnome.system.proxy.http\x00host":      `'corp.example'`,
			"org.gnome.system.proxy.http\x00port":      `uint32 8080`,
			"org.gnome.system.proxy.https\x00host":     `'secure.corp'`,
			"org.gnome.system.proxy.https\x00port":     `uint32 8443`,
			"org.gnome.system.proxy.socks\x00host":     `'socks.corp'`,
			"org.gnome.system.proxy.socks\x00port":     `uint32 1080`,
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
		if parseGsettingsUInt(port) != 8080 {
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

type failingGSettings struct {
	*mapGSettings
	setErr error
}

func (f *failingGSettings) Set(schema, key, value string) error {
	if f.setErr != nil {
		return f.setErr
	}
	return f.mapGSettings.Set(schema, key, value)
}

func TestLinuxPartialRestoreConflictAndFreshManagerRecovery(t *testing.T) {
	t.Setenv("XDG_CURRENT_DESKTOP", "GNOME")
	runner := &failingGSettings{mapGSettings: &mapGSettings{data: map[string]string{
		"org.gnome.system.proxy\x00mode":           `'manual'`,
		"org.gnome.system.proxy.http\x00host":      `'corp.example'`,
		"org.gnome.system.proxy.http\x00port":      `uint32 8080`,
		"org.gnome.system.proxy.https\x00host":     `'secure.corp'`,
		"org.gnome.system.proxy.https\x00port":     `uint32 8443`,
		"org.gnome.system.proxy.socks\x00host":     `'socks.corp'`,
		"org.gnome.system.proxy.socks\x00port":     `uint32 1080`,
		"org.gnome.system.proxy\x00ignore-hosts":   `['corp.lan']`,
		"org.gnome.system.proxy\x00autoconfig-url": `''`,
	}}}
	journalPath := filepath.Join(t.TempDir(), "proxy-recovery.json")
	m1 := New(LinuxBackend{Runner: runner}, journalPath)
	if err := m1.Apply(7890); err != nil {
		t.Fatal(err)
	}

	// (c) Simulate a failed restore first: journal must be preserved.
	runner.setErr = errors.New("dconf write error")
	if _, err := m1.Restore(); err == nil {
		t.Fatal("expected restore error when gsettings Set fails")
	}
	if raw, err := os.ReadFile(journalPath); err != nil || len(raw) == 0 {
		t.Fatalf("journal must remain on restore error: len=%d err=%v", len(raw), err)
	}

	// User externally edits https host/port while recovery is pending.
	runner.setErr = nil
	_ = runner.mapGSettings.Set("org.gnome.system.proxy.https", "host", `'user-https.local'`)
	_ = runner.mapGSettings.Set("org.gnome.system.proxy.https", "port", `uint32 9443`)

	// (c) Fresh Manager recovers still-owned leftovers, (a) removes journal, (b) preserves external edit.
	m2 := New(LinuxBackend{Runner: runner}, journalPath)
	if pending, err := m2.Pending(); err != nil || !pending {
		t.Fatalf("fresh Manager Pending=%v err=%v, want true", pending, err)
	}
	conflicts, err := m2.Restore()
	if err != nil {
		t.Fatalf("fresh Manager Restore failed: %v", err)
	}
	if len(conflicts) == 0 {
		t.Fatal("expected conflict for externally edited HTTPS group")
	}
	httpHost, _ := runner.Get("org.gnome.system.proxy.http", "host")
	assertGsettingsString(t, httpHost, `'corp.example'`)
	httpsHost, _ := runner.Get("org.gnome.system.proxy.https", "host")
	assertGsettingsString(t, httpsHost, `'user-https.local'`)
	if _, err := os.Stat(journalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("journal must be removed after partial restore, err=%v", err)
	}
	// Ownership released: second Restore does not touch external edit.
	if conflicts2, err := New(LinuxBackend{Runner: runner}, journalPath).Restore(); err != nil || len(conflicts2) != 0 {
		t.Fatalf("second Restore should be no-op: conflicts=%v err=%v", conflicts2, err)
	}
	httpsHost, _ = runner.Get("org.gnome.system.proxy.https", "host")
	assertGsettingsString(t, httpsHost, `'user-https.local'`)

	// (d) Apply -> Restore -> Apply again works.
	if err := m2.Apply(7891); err != nil {
		t.Fatalf("re-apply after partial restore failed: %v", err)
	}
	httpPort, _ := runner.Get("org.gnome.system.proxy.http", "port")
	if httpPort != "uint32 7891" && httpPort != "7891" {
		t.Fatalf("re-apply HTTP port=%q, want 7891", httpPort)
	}
}
