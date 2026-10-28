//go:build darwin

package core

import (
	"os"
	"testing"
)

func TestKernelRestartTun(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO")
	}
	s := New()
	s.proxy = &fakeProxy{}
	s.tunLaunch = fakeTunLaunch
	s.tunReady = func(map[string]bool) bool { return true }
	defer s.Shutdown()
	if err := s.SetTun(true); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(direct, freePort(t)); err != nil {
		t.Fatal(err)
	}
	before := s.Status()
	if err := s.Restart(); err != nil {
		t.Fatal(err)
	}
	after := s.Status()
	if !after.Running || !after.Tun || after.SystemProxy || !after.StartedAt.After(before.StartedAt) {
		t.Fatalf("TUN restart: %+v", after)
	}
}
