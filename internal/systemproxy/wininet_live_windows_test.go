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
	if afterReg.AutoDiscovery {
		t.Fatal("registry AutoDetect/WPAD flag must be off during takeover (distinct from ProxySettingsPerUser policy)")
	}
	apiServer, _, apiEnabled, err := queryWinINETProxyViaAPI()
	if err != nil {
		t.Fatal(err)
	}
	if !apiEnabled || !strings.Contains(apiServer, "127.0.0.1:7890") {
		t.Fatalf("WinINET API proxy=%q enabled=%v", apiServer, apiEnabled)
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
	_, _, apiEnabledAfter, err := queryWinINETProxyViaAPI()
	if err != nil {
		t.Fatal(err)
	}
	if apiEnabledAfter != (before.ProxyEnable && before.ProxyServer != "") {
		t.Fatalf("WinINET API still enabled=%v after restore", apiEnabledAfter)
	}
}
