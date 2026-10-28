//go:build darwin

package core

import (
	"os"
	"testing"
)

func TestStartupAutoConnectTunUsesSavedChoice(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO")
	}
	s := New()
	s.proxy = &fakeProxy{}
	s.geo.dir = t.TempDir()
	s.tunLaunch = fakeTunLaunch
	s.tunReady = func(map[string]bool) bool { return true }
	defer s.Shutdown()
	p := NewProfilesWithService(t.TempDir(), s)
	if _, err := p.Save(Profile{Name: "auto", Source: "file", Content: direct}, ""); err != nil {
		t.Fatal(err)
	}
	if err := p.SetPort(freePort(t)); err != nil {
		t.Fatal(err)
	}
	if err := p.ChangeTun(true); err != nil {
		t.Fatal(err)
	}
	if err := p.SetAutoConnect(true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTun(false); err != nil {
		t.Fatal(err)
	}
	if err := p.Initialize(); err != nil {
		t.Fatal(err)
	}
	if st := s.Status(); !st.Running || !st.Tun || st.SystemProxy {
		t.Fatalf("auto TUN: %+v", st)
	}
}
