package core

import (
	"os"
	"reflect"
	"testing"
)

func TestProxyBypassPersistenceAndValidation(t *testing.T) {
	p := NewProfiles(t.TempDir())
	first, err := p.ProxyBypass()
	if err != nil || first.Custom {
		t.Fatalf("default: %+v %v", first, err)
	}
	want := []string{"*.local", "192.168.0.0/16"}
	got, err := p.SetProxyBypass(ProxyBypassSettings{Custom: true, Entries: []string{" *.local ", "*.LOCAL", "", want[1]}})
	if err != nil || !reflect.DeepEqual(got.Entries, want) {
		t.Fatalf("save: %+v %v", got, err)
	}
	loaded, err := NewProfiles(p.dir).ProxyBypass()
	if err != nil || !loaded.Custom || !reflect.DeepEqual(loaded.Entries, want) {
		t.Fatalf("reload: %+v %v", loaded, err)
	}
	if _, err = p.SetProxyBypass(ProxyBypassSettings{Custom: true, Entries: []string{"https://example.com"}}); err == nil {
		t.Fatal("accepted URL")
	}
	loaded, _ = p.ProxyBypass()
	if !reflect.DeepEqual(loaded.Entries, want) {
		t.Fatal("invalid input overwrote saved values")
	}
	got, err = p.SetProxyBypass(ProxyBypassSettings{Custom: true, Entries: []string{}})
	if err != nil || !got.Custom || len(got.Entries) != 0 {
		t.Fatalf("explicit empty: %+v %v", got, err)
	}
	got, err = p.SetProxyBypass(ProxyBypassSettings{})
	if err != nil || got.Custom {
		t.Fatalf("system defaults: %+v %v", got, err)
	}
}
func TestProxyBypassFlowsIntoCoreTakeover(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO to test core")
	}
	proxy := &fakeProxy{}
	s := New()
	s.proxy = proxy
	defer s.Shutdown()
	if err := s.SetSystemProxy(true); err != nil {
		t.Fatal(err)
	}
	p := NewProfilesWithService(t.TempDir(), s)
	if _, err := p.SetProxyBypass(ProxyBypassSettings{Custom: true, Entries: []string{"*.local"}}); err != nil {
		t.Fatal(err)
	}
	if err := p.StartDraft(direct, freePort(t)); err != nil {
		t.Fatal(err)
	}
	if proxy.bypass == nil || !reflect.DeepEqual(*proxy.bypass, []string{"*.local"}) {
		t.Fatalf("missing bypass: %v", proxy.bypass)
	}
	if _, err := p.SetProxyBypass(ProxyBypassSettings{Custom: false}); err == nil {
		t.Fatal("expected SetProxyBypass to be rejected while core is running")
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
}
