package core

import (
	"strings"
	"testing"

	"rocketscience/internal/systemproxy"
)

func TestStartupPreferencesAndMissingTakeover(t *testing.T) {
	s := New()
	s.proxy = &fakeProxy{}
	p := NewProfilesWithService(t.TempDir(), s)
	withAutomaticSystemProxy(t)
	if err := p.SetSystemProxy(false); err != nil {
		t.Fatal(err)
	}
	if err := p.SetAutoConnect(true); err != nil {
		t.Fatal(err)
	}
	if enabled, err := NewProfiles(p.dir).AutoConnect(); err != nil || !enabled {
		t.Fatalf("saved: %v %v", enabled, err)
	}
	if err := p.Initialize(); err == nil {
		if takeoverRequiredBeforeStart() {
			t.Fatalf("missing takeover: %v", err)
		}
	} else if takeoverRequiredBeforeStart() && !strings.Contains(err.Error(), "接管方式") {
		t.Fatalf("missing takeover: %v", err)
	}
	if takeoverRequiredBeforeStart() && s.Status().Running {
		t.Fatal("started without takeover")
	}
	if err := p.SetAutoConnect(false); err != nil {
		t.Fatal(err)
	}
	if err := p.SetSystemProxy(true); err != nil {
		t.Fatal(err)
	}
	if err := p.Initialize(); err != nil {
		t.Fatal(err)
	}
	st := s.Status()
	if st.Running || !st.SystemProxy {
		t.Fatalf("saved choice: %+v", st)
	}
}
func TestNewInstallDefaultsToSystemProxyAndKeepsSavedChoice(t *testing.T) {
	withAutomaticSystemProxy(t)
	p := NewProfiles(t.TempDir())
	summary, err := p.Summary()
	wantDefault := systemproxy.AutomaticAvailable()
	if err != nil || summary.SystemProxy != wantDefault {
		t.Fatalf("new install: %+v %v want %v", summary, err, wantDefault)
	}
	if err = p.SetSystemProxy(false); err != nil {
		t.Fatal(err)
	}
	summary, err = NewProfiles(p.dir).Summary()
	if err != nil || summary.SystemProxy {
		t.Fatalf("saved choice: %+v %v", summary, err)
	}
}
func TestStartupAutomaticConnectAndRecoveryGuard(t *testing.T) {
	withAutomaticSystemProxy(t)
	t.Setenv("ROCKETSCIENCE_MIHOMO", t.TempDir()+"/missing")
	s := New()
	proxy := &fakeProxy{}
	s.proxy = proxy
	s.geo.dir = t.TempDir()
	defer s.Shutdown()
	p := NewProfilesWithService(t.TempDir(), s)
	if _, err := p.Save(Profile{Name: "auto", Source: "file", Content: direct}, ""); err != nil {
		t.Fatal(err)
	}
	if err := p.SetPort(freePort(t)); err != nil {
		t.Fatal(err)
	}
	if err := p.SetSystemProxy(true); err != nil {
		t.Fatal(err)
	}
	if err := p.SetAutoConnect(true); err != nil {
		t.Fatal(err)
	}
	proxy.pending = true
	if err := p.Initialize(); err == nil || !strings.Contains(err.Error(), "恢复") {
		t.Fatalf("pending recovery: %v", err)
	}
	if s.Status().Running {
		t.Fatal("started with pending recovery")
	}
	proxy.pending = false
	if err := p.Initialize(); err != nil {
		t.Fatal(err)
	}
	if st := s.Status(); !st.Running || !st.SystemProxy || !proxy.pending {
		t.Fatalf("auto start: %+v", st)
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
}
func TestStartupAutomaticRequiresSavedProfile(t *testing.T) {
	withAutomaticSystemProxy(t)
	s := New()
	s.proxy = &fakeProxy{}
	p := NewProfilesWithService(t.TempDir(), s)
	if err := p.SetSystemProxy(true); err != nil {
		t.Fatal(err)
	}
	if err := p.SetAutoConnect(true); err != nil {
		t.Fatal(err)
	}
	if err := p.Initialize(); err == nil || !strings.Contains(err.Error(), "配置") {
		t.Fatalf("missing profile: %v", err)
	}
	if s.Status().Running {
		t.Fatal("started without saved profile")
	}
}
