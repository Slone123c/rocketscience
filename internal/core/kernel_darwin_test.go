//go:build darwin

package core

import (
	"os"
	"path/filepath"
	"strings"
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

func TestPathsWithSpacesChineseAndSingleQuotes(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO")
	}
	specialDir := filepath.Join(t.TempDir(), "Clash 测试 '目录' 2026")
	quoted, err := shellArgument(specialDir)
	if err != nil || !strings.Contains(quoted, `'\''`) {
		t.Fatalf("shellArgument(%q): got %q, %v", specialDir, quoted, err)
	}
	s := newTestService(t)
	s.geo.dir = filepath.Join(specialDir, "geo")
	defer s.Shutdown()
	p := NewProfilesWithService(filepath.Join(specialDir, "profiles"), s)
	if err := p.SetPort(freePort(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Save(Profile{Name: "特殊路径配置", Source: "file", Content: direct}, ""); err != nil {
		t.Fatal(err)
	}
	if err := p.StartActive(); err != nil {
		t.Fatalf("StartActive under special directory failed: %v", err)
	}
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop under special directory failed: %v", err)
	}
}
