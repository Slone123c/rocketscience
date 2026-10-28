package core

import (
	"gopkg.in/yaml.v3"
	"os/exec"
	"testing"
)

func TestTunOptionsPersistenceValidationAndMerging(t *testing.T) {
	s := New()
	p := NewProfilesWithService(t.TempDir(), s)
	value := TunOptions{Stack: ptr("system"), AutoRoute: ptr(false), AutoDetectInterface: ptr(false), StrictRoute: ptr(true), MTU: ptr(1400), DNSHijack: ptr([]string{})}
	if _, err := p.SetTunSettings(value); err != nil {
		t.Fatal(err)
	}
	loaded, err := NewProfiles(p.dir).TunSettings()
	if err != nil || loaded.Stack == nil || *loaded.Stack != "system" || loaded.DNSHijack == nil || len(*loaded.DNSHijack) != 0 {
		t.Fatalf("persisted: %+v %v", loaded, err)
	}
	if _, err := p.SetCoreSettings(CoreOptions{LogLevel: ptr("warning")}); err != nil {
		t.Fatal(err)
	}
	options, err := p.CoreSettings()
	if err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{false, true} {
		b, err := runtimeConfigWithOptions(direct, 7890, 9999, "secret", "", LANSettings{}, enabled, options)
		if err != nil {
			t.Fatal(err)
		}
		var cfg map[string]any
		if err = yaml.Unmarshal(b, &cfg); err != nil {
			t.Fatal(err)
		}
		tun := cfg["tun"].(map[string]any)
		if tun["enable"] != enabled {
			t.Fatal("advanced options changed takeover selection")
		}
		if enabled && (tun["mtu"] != 1400 || tun["auto-route"] != false || tun["stack"] != "system" || len(tun["dns-hijack"].([]any)) != 0) {
			t.Fatalf("merged: %+v", tun)
		}
	}
	for _, bad := range []TunOptions{{Stack: ptr("invalid")}, {MTU: ptr(575)}, {MTU: ptr(65536)}, {DNSHijack: ptr([]string{"example.com:53"})}, {DNSHijack: ptr([]string{"any:70000"})}, {DNSHijack: ptr([]string{"https://any:53"})}} {
		if _, err := p.SetTunSettings(bad); err == nil {
			t.Fatalf("invalid: %+v", bad)
		}
	}
	if err := validateTunOptions(TunOptions{DNSHijack: ptr([]string{"any:53", "tcp://[::1]:53", "udp://127.0.0.1:1053"})}); err != nil {
		t.Fatal(err)
	}
	s.cmd = &exec.Cmd{}
	if _, err := p.SetTunSettings(TunOptions{}); err == nil {
		t.Fatal("changed virtual adapter settings while running")
	}
	s.cmd = nil
}
