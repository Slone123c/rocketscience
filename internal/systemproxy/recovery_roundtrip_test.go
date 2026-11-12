package systemproxy

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRecoveryPreservesNonLoopbackAndPAC(t *testing.T) {
	original := Settings{
		"__ProtocolEnabled":        json.RawMessage("false"),
		"__WinINET.ProxyServer":    json.RawMessage(`"http=corp.example:8080;https=secure.corp:8443;socks=socks.corp:1080"`),
		"__WinINET.AutoConfigURL":  json.RawMessage(`"http://pac.corp/proxy.pac"`),
		"HTTPEnable":               json.RawMessage("0"),
		"HTTPProxy":                json.RawMessage(`"corp.example"`),
		"HTTPPort":                 json.RawMessage("8080"),
		"HTTPSEnable":              json.RawMessage("0"),
		"HTTPSProxy":               json.RawMessage(`"secure.corp"`),
		"HTTPSPort":                json.RawMessage("8443"),
		"SOCKSEnable":              json.RawMessage("0"),
		"SOCKSProxy":               json.RawMessage(`"socks.corp"`),
		"SOCKSPort":                json.RawMessage("1080"),
		"ProxyAutoConfigEnable":    json.RawMessage("1"),
		"ProxyAutoDiscoveryEnable": json.RawMessage("1"),
		"ExceptionsList":           json.RawMessage(`["intranet.corp"]`),
	}
	b := &fakeBackend{services: []Service{{ID: "wininet", Settings: cloneSettings(original)}}}
	path := filepath.Join(t.TempDir(), "journal")
	m := New(b, path)
	if err := m.Apply(7890); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Restore(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pickKeys(b.services[0].Settings, keys), pickKeys(original, keys)) {
		t.Fatalf("restored keys mismatch:\n%v\n%v", b.services[0].Settings, original)
	}
	if parseString(b.services[0].Settings["HTTPProxy"]) != "corp.example" || parsePort(b.services[0].Settings["HTTPPort"]) != 8080 {
		t.Fatalf("non-loopback proxy not restored: %v", b.services[0].Settings)
	}
	if !parseBool(b.services[0].Settings["ProxyAutoConfigEnable"]) {
		t.Fatalf("PAC enable not restored")
	}
}

func cloneSettings(in Settings) Settings {
	out := Settings{}
	for k, v := range in {
		out[k] = append(json.RawMessage(nil), v...)
	}
	return out
}
