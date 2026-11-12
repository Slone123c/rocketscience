//go:build !darwin

package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rocketscience/internal/systemproxy"
)

func TestUnsupportedSessionClearsPersistedSystemProxyAndManualStart(t *testing.T) {
	t.Setenv("XDG_CURRENT_DESKTOP", "KDE")
	systemproxy.SetAutomaticAvailableOverride(func() (bool, bool) { return false, true })
	t.Cleanup(func() { systemproxy.SetAutomaticAvailableOverride(nil) })

	dir := t.TempDir()
	index := profileData{
		Profiles:    []Profile{{ID: "active", Name: "active", Source: "file", Content: direct}},
		ActiveID:    "active",
		MixedPort:   freePort(t),
		SystemProxy: true,
	}
	b, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "profiles.json"), b, 0600); err != nil {
		t.Fatal(err)
	}

	proxy := &fakeProxy{}
	s := New()
	s.proxy = proxy
	s.geo.dir = t.TempDir()
	t.Setenv("ROCKETSCIENCE_MIHOMO", filepath.Join(t.TempDir(), "missing"))
	defer s.Shutdown()

	p := NewProfilesWithService(dir, s)
	if err := p.Initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if proxy.pending {
		t.Fatal("initialize must not apply system proxy when automatic takeover is unavailable")
	}
	if st := s.Status(); st.SystemProxy {
		t.Fatal("runtime system proxy flag must be false")
	}
	state, err := p.State()
	if err != nil || state.SystemProxy {
		t.Fatalf("profile state: %+v %v", state, err)
	}
	onDisk, err := os.ReadFile(filepath.Join(dir, "profiles.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(onDisk), `"systemProxy":true`) {
		t.Fatal("persisted profile must store systemProxy false after initialize")
	}
	if err := p.StartActive(); err != nil {
		t.Fatalf("manual start: %v", err)
	}
	if proxy.pending {
		t.Fatal("start must not touch OS proxy in manual mode")
	}
	if !s.Status().Running {
		t.Fatal("core should run without takeover")
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if proxy.pending {
		t.Fatal("stop must not apply proxy in manual mode")
	}
}
