package core

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestTunDNSEnablesResolverAndPreservesUpstreams(t *testing.T) {
	content := direct + "dns:\n  enable: false\n  enhanced-mode: redir-host\n  nameserver: [udp://127.0.0.1:5353]\n  default-nameserver: [1.1.1.1]\n  nameserver-policy: {example.com: 9.9.9.9}\n"
	off := false
	data, err := runtimeConfigWithOptions(content, 17890, 19090, "test", "", LANSettings{}, true, CoreOptions{DNS: &DNSOptions{Enabled: &off}})
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		t.Fatal(err)
	}
	dns := cfg["dns"].(map[string]any)
	if dns["enable"] != true || dns["enhanced-mode"] != "redir-host" || !reflect.DeepEqual(dns["nameserver"], []any{"udp://127.0.0.1:5353"}) || !reflect.DeepEqual(dns["default-nameserver"], []any{"1.1.1.1"}) {
		t.Fatalf("DNS upstreams changed: %+v", dns)
	}
	if dns["nameserver-policy"].(map[string]any)["example.com"] != "9.9.9.9" {
		t.Fatal("nameserver policy lost")
	}
	data, err = runtimeConfigWithOptions(content, 17890, 19090, "test", "", LANSettings{}, false, CoreOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_ = yaml.Unmarshal(data, &cfg)
	if cfg["dns"].(map[string]any)["enable"] != false {
		t.Fatal("non-TUN configuration changed")
	}
}

func TestTunDNSDefaultConfigPassesRealCoreValidation(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO for real config validation")
	}
	data, err := runtimeConfigWithOptions(direct, 17890, 19090, "test", "", LANSettings{}, true, CoreOptions{})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := validateRuntimeWithContext(ctx, corePath(), dir, path); err != nil {
		t.Fatal(err)
	}
}

func TestTunDNSPreferencesLinkage(t *testing.T) {
	p := NewProfilesWithService(t.TempDir(), newTestService(t))
	off := false
	upstream := []string{"https://custom.example/dns-query"}
	if err := p.write(profileData{CoreOptions: &CoreOptions{DNS: &DNSOptions{Enabled: &off, Nameservers: &upstream}}}); err != nil {
		t.Fatal(err)
	}
	if err := p.ChangeTun(true); err != nil {
		t.Fatal(err)
	}
	d, err := p.load()
	if err != nil {
		t.Fatal(err)
	}
	if !d.Tun || d.CoreOptions.DNS.Enabled == nil || !*d.CoreOptions.DNS.Enabled || !reflect.DeepEqual(*d.CoreOptions.DNS.Nameservers, upstream) {
		t.Fatalf("preferences not linked: %+v", d.CoreOptions.DNS)
	}
	if _, err := p.SetDNSSettings(DNSOptions{Enabled: &off}); err == nil {
		t.Fatal("allowed DNS shutdown with TUN still enabled")
	}
	if err := p.ChangeTun(false); err != nil {
		t.Fatal(err)
	}
	d, err = p.load()
	if err != nil {
		t.Fatal(err)
	}
	if d.Tun || !*d.CoreOptions.DNS.Enabled {
		t.Fatal("turning TUN off should retain saved DNS preference")
	}
}
