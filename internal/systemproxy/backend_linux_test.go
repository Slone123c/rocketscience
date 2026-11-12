//go:build linux

package systemproxy

import (
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
	m.data[m.key(schema, key)] = value
	return nil
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
