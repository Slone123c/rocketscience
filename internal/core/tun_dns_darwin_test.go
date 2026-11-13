//go:build darwin

package core

import (
	"reflect"
	"testing"
)

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
