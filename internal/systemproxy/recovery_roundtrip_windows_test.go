//go:build windows

package systemproxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type memWinINET struct {
	snap winINETSnapshot
}

func (m *memWinINET) Read() (winINETSnapshot, error)  { return m.snap, nil }
func (m *memWinINET) Write(s winINETSnapshot) error   { m.snap = s; return nil }
func (m *memWinINET) Notify() error                   { return nil }

func winINETEqual(a, b winINETSnapshot) bool {
	return reflect.DeepEqual(a, b)
}

func TestJournalAcceptsWinINETNativeFields(t *testing.T) {
	changes := []Change{{
		ID:       winINETServiceID,
		Keys:     []string{winINETKeyProxyServerRaw, winINETKeyAutoConfigURL},
		Expected: Settings{winINETKeyProxyServerRaw: json.RawMessage(`"http=corp:1"`), winINETKeyAutoConfigURL: json.RawMessage(`"http://pac"`)},
		Desired:  Settings{winINETKeyProxyServerRaw: json.RawMessage(`"http=127.0.0.1:9"`), winINETKeyAutoConfigURL: json.RawMessage(`""`)},
	}}
	if err := validateJournalChanges(changes); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsPartialRestoreExternalPACOnly(t *testing.T) {
	original := winINETSnapshot{
		ProxyEnable:   false,
		ProxyServer:   "http=corp.example:8080",
		AutoConfigURL: "http://pac.corp/proxy.pac",
	}
	mem := &memWinINET{snap: original}
	journal := filepath.Join(t.TempDir(), "proxy-recovery.json")
	if err := New(WindowsBackend{Access: mem}, journal).Apply(7890); err != nil {
		t.Fatal(err)
	}
	mem.snap.AutoConfigURL = "http://user-edited.pac"
	conflicts, err := New(WindowsBackend{Access: mem}, journal).Restore()
	if err != nil {
		t.Fatal(err)
	}
	if len(conflicts) == 0 {
		t.Fatal("expected PAC native group conflict")
	}
	if mem.snap.ProxyServer != original.ProxyServer {
		t.Fatalf("ProxyServer=%q want %q", mem.snap.ProxyServer, original.ProxyServer)
	}
	if mem.snap.AutoConfigURL != "http://user-edited.pac" {
		t.Fatalf("external PAC kept: %q", mem.snap.AutoConfigURL)
	}
	if _, err := os.Stat(journal); err != nil {
		t.Fatal("journal must remain after partial restore")
	}
}

func TestWindowsBackendRecoveryRoundTrip_PACAndDisabledProxy(t *testing.T) {
	original := winINETSnapshot{
		ProxyEnable:   false,
		ProxyServer:   "http=corp.example:8080;https=secure.corp:8443;socks=socks.corp:1080",
		ProxyOverride: "intranet.corp",
		AutoConfigURL: "http://pac.corp/proxy.pac",
		AutoDiscovery: true,
	}
	mem := &memWinINET{snap: original}
	journal := filepath.Join(t.TempDir(), "proxy-recovery.json")
	m1 := New(WindowsBackend{Access: mem}, journal)
	if err := m1.Apply(7890); err != nil {
		t.Fatal(err)
	}
	afterTakeover, err := mem.Read()
	if err != nil {
		t.Fatal(err)
	}
	if afterTakeover.ProxyServer != proxyServerLine(7890) {
		t.Fatalf("takeover ProxyServer=%q", afterTakeover.ProxyServer)
	}
	if afterTakeover.AutoConfigURL != "" {
		t.Fatalf("takeover cleared PAC URL expected empty, got %q", afterTakeover.AutoConfigURL)
	}

	m2 := New(WindowsBackend{Access: mem}, journal)
	if conflicts, err := m2.Restore(); err != nil {
		t.Fatalf("restore: %v conflicts=%v", err, conflicts)
	} else if len(conflicts) > 0 {
		t.Fatalf("conflicts=%v", conflicts)
	}
	restored, err := mem.Read()
	if err != nil {
		t.Fatal(err)
	}
	if !winINETEqual(restored, original) {
		t.Fatalf("native snapshot mismatch:\nhave %+v\nwant %+v", restored, original)
	}
	services, err := WindowsBackend{Access: mem}.Snapshot()
	if err != nil || len(services) != 1 {
		t.Fatalf("snapshot after restore: %v %v", services, err)
	}
	settings := services[0].Settings
	if parseString(settings[winINETKeyAutoConfigURL]) != original.AutoConfigURL {
		t.Fatalf("settings PAC URL=%q", settings[winINETKeyAutoConfigURL])
	}
	if parseString(settings[winINETKeyProxyServerRaw]) != original.ProxyServer {
		t.Fatalf("settings ProxyServer=%q", settings[winINETKeyProxyServerRaw])
	}
	if _, ok := settings[winINETKeyProxyServerRaw]; !ok {
		t.Fatal("missing raw ProxyServer in fresh snapshot")
	}
}
