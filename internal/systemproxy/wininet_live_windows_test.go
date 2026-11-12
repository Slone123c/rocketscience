//go:build windows

package systemproxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LiveWinINET mutates the ephemeral CI runner's HKCU Internet Settings only (never a developer host).
func TestLiveWinINETApplyRestoreOnRunner(t *testing.T) {
	if os.Getenv("GITHUB_ACTIONS") != "true" {
		t.Skip("live WinINET roundtrip runs only on ephemeral GitHub-hosted Windows runners")
	}
	access := registryWinINET{}
	before, err := access.Read()
	if err != nil {
		t.Fatal(err)
	}
	perUserBefore, perUserPresent, _ := readWinINETRegistryDword("ProxySettingsPerUser")
	t.Cleanup(func() {
		_ = access.Write(before)
		_ = access.Notify()
	})

	journal := filepath.Join(t.TempDir(), "proxy-recovery.json")
	backend := WindowsBackend{Access: access}
	if err := New(backend, journal).Apply(7890); err != nil {
		t.Fatalf("apply: %v", err)
	}
	afterReg, err := access.Read()
	if err != nil {
		t.Fatal(err)
	}
	if afterReg.ProxyServer != proxyServerLine(7890) {
		t.Fatalf("registry ProxyServer=%q", afterReg.ProxyServer)
	}
	live, err := queryWinINETUserSettings()
	if err != nil {
		t.Fatal(err)
	}
	if live.AutoDetectEnabled() {
		t.Fatalf("WinINET per-connection flags still have AUTO_DETECT (WPAD): %#x", live.Flags)
	}
	if live.AutoProxyURLEnabled() && strings.TrimSpace(live.AutoConfigURL) != "" {
		t.Fatalf("WinINET per-connection PAC still active url=%q flags=%#x", live.AutoConfigURL, live.Flags)
	}
	if !live.ProxyEnabled() || !strings.Contains(live.ProxyServer, "127.0.0.1:7890") {
		t.Fatalf("WinINET per-connection proxy=%q enabled=%v flags=%#x", live.ProxyServer, live.ProxyEnabled(), live.Flags)
	}
	if perUserPresent {
		perUserAfter, stillPresent, err := readWinINETRegistryDword("ProxySettingsPerUser")
		if err != nil || !stillPresent || perUserAfter != perUserBefore {
			t.Fatalf("must not rewrite ProxySettingsPerUser (per-user policy); before=%d after=%v", perUserBefore, perUserAfter)
		}
	}

	m2 := New(WindowsBackend{Access: access}, journal)
	if conflicts, err := m2.Restore(); err != nil {
		t.Fatalf("restore: %v conflicts=%v", err, conflicts)
	}
	restored, err := access.Read()
	if err != nil {
		t.Fatal(err)
	}
	if !winINETEqual(restored, before) {
		t.Fatalf("registry restore mismatch:\nhave %+v\nwant %+v", restored, before)
	}
	afterLive, err := queryWinINETUserSettings()
	if err != nil {
		t.Fatal(err)
	}
	wantProxy := before.ProxyEnable && before.ProxyServer != ""
	if afterLive.ProxyEnabled() != wantProxy {
		t.Fatalf("WinINET per-connection proxy enabled=%v want=%v flags=%#x", afterLive.ProxyEnabled(), wantProxy, afterLive.Flags)
	}
	if afterLive.AutoDetectEnabled() != before.AutoDiscovery {
		t.Fatalf("WinINET per-connection AUTO_DETECT=%v want=%v flags=%#x", afterLive.AutoDetectEnabled(), before.AutoDiscovery, afterLive.Flags)
	}
	if wantProxy && afterLive.ProxyServer != before.ProxyServer {
		t.Fatalf("WinINET per-connection proxy server=%q want=%q", afterLive.ProxyServer, before.ProxyServer)
	}
}
