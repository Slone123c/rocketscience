//go:build linux

package systemproxy

import (
	"strings"
	"sync"
	"testing"
)

type mapGSettings struct {
	mu   sync.Mutex
	data map[string]string
}

func (m *mapGSettings) key(schema, name string) string { return schema + "\x00" + name }

func (m *mapGSettings) Get(schema, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.data[m.key(schema, key)], nil
}

func (m *mapGSettings) Set(schema, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		m.data = map[string]string{}
	}
	m.data[m.key(schema, key)] = canonicalGsettingsRaw(key, value)
	return nil
}

// canonicalGsettingsRaw normalizes gsettings literals so fake-runner reads match real Set() output.
func canonicalGsettingsRaw(key, value string) string {
	switch key {
	case "mode", "host", "autoconfig-url":
		parsed, err := parseGsettingsValue(value)
		if err != nil {
			return value
		}
		return formatGsettingsString(parsed)
	case "ignore-hosts":
		items, err := parseGsettingsStringArray(value)
		if err != nil {
			return value
		}
		return formatGsettingsStringArray(items)
	case "port":
		return strings.TrimSpace(value)
	default:
		return value
	}
}

func assertGsettingsString(t *testing.T, got, want string) {
	t.Helper()
	g, err := parseGsettingsValue(got)
	if err != nil {
		t.Fatalf("parse got %q: %v", got, err)
	}
	w, err := parseGsettingsValue(want)
	if err != nil {
		t.Fatalf("parse want %q: %v", want, err)
	}
	if g != w {
		t.Fatalf("gsettings string got=%q (%q) want=%q (%q)", got, g, want, w)
	}
}

func assertGsettingsStringArray(t *testing.T, got, want string) {
	t.Helper()
	g, err := parseGsettingsStringArray(got)
	if err != nil {
		t.Fatalf("parse got %q: %v", got, err)
	}
	w, err := parseGsettingsStringArray(want)
	if err != nil {
		t.Fatalf("parse want %q: %v", want, err)
	}
	if len(g) != len(w) {
		t.Fatalf("ignore-hosts len %d want %d (got=%q want=%q)", len(g), len(w), got, want)
	}
	for i := range g {
		if g[i] != w[i] {
			t.Fatalf("ignore-hosts[%d]=%q want %q (got=%q want=%q)", i, g[i], w[i], got, want)
		}
	}
}

func TestGnomeCompareAndSwapAppliesLoopbackProxy(t *testing.T) {
	t.Setenv("XDG_CURRENT_DESKTOP", "GNOME")
	runner := &mapGSettings{data: map[string]string{
		"org.gnome.system.proxy\x00mode": `'none'`,
	}}
	backend := LinuxBackend{Runner: runner}
	services, err := backend.Snapshot()
	if err != nil || len(services) != 1 {
		t.Fatalf("snapshot: %v %v", services, err)
	}
	port := 7890
	desired := takeover(port)
	expected := pickKeys(services[0].Settings, []string{
		"HTTPEnable", "HTTPProxy", "HTTPPort", "HTTPSEnable", "HTTPSProxy", "HTTPSPort",
		"SOCKSEnable", "SOCKSProxy", "SOCKSPort", "ProxyAutoConfigEnable", "ProxyAutoDiscoveryEnable", "__ProtocolEnabled",
	})
	changes := []Change{{
		ID:       services[0].ID,
		Expected: expected,
		Desired:  desired,
		Keys: []string{
			"HTTPEnable", "HTTPProxy", "HTTPPort", "HTTPSEnable", "HTTPSProxy", "HTTPSPort",
			"SOCKSEnable", "SOCKSProxy", "SOCKSPort", "ProxyAutoConfigEnable", "ProxyAutoDiscoveryEnable", "__ProtocolEnabled",
		},
	}}
	conflicts, err := backend.CompareAndSwap(changes)
	if err != nil || len(conflicts) > 0 {
		t.Fatalf("cas: %v %v", conflicts, err)
	}
	mode, _ := runner.Get("org.gnome.system.proxy", "mode")
	if mode != `"manual"` {
		t.Fatalf("mode=%q", mode)
	}
	host, _ := runner.Get("org.gnome.system.proxy.http", "host")
	if host != `"127.0.0.1"` {
		t.Fatalf("host=%q", host)
	}
}
