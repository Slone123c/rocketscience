//go:build darwin

package core

import (
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"testing"
)

func TestTunOptionsDeliveredAtStartup(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", t.TempDir()+"/missing")
	s := New()
	s.proxy = &fakeProxy{}
	s.geo.dir = testGeoDir(t)
	s.tunReady = func(map[string]bool) bool { return true }
	defer s.Shutdown()
	var received map[string]any
	s.tunLaunch = func(core, dir string) (*tunLauncher, error) {
		b, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
		if err != nil {
			return nil, err
		}
		if err = yaml.Unmarshal(b, &received); err != nil {
			return nil, err
		}
		return fakeTunLaunch(core, dir)
	}
	p := NewProfilesWithService(t.TempDir(), s)
	if _, err := p.SetTunSettings(TunOptions{Stack: ptr("mixed"), MTU: ptr(1500), DNSHijack: ptr([]string{"any:53"})}); err != nil {
		t.Fatal(err)
	}
	if err := p.ChangeTun(true); err != nil {
		t.Fatal(err)
	}
	if err := p.StartDraft(direct, freePort(t)); err != nil {
		t.Fatal(err)
	}
	tun := received["tun"].(map[string]any)
	if tun["enable"] != true || tun["mtu"] != 1500 || tun["stack"] != "mixed" {
		t.Fatalf("delivered: %+v", tun)
	}
	if _, err := p.SetTunSettings(TunOptions{}); err == nil {
		t.Fatal("changed live TUN parameters")
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if _, err := p.SetTunSettings(TunOptions{}); err != nil {
		t.Fatal(err)
	}
}
