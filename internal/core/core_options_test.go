package core

import (
	"net/http"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func ptr[T any](v T) *T { return &v }
func TestCoreOptionsPersistenceAndValidation(t *testing.T) {
	p := NewProfiles(t.TempDir())
	defaults, err := p.CoreSettings()
	if err != nil || defaults.IPv6 != nil || defaults.LogLevel != nil {
		t.Fatalf("defaults: %+v %v", defaults, err)
	}
	want := CoreOptions{LogLevel: ptr("debug"), IPv6: ptr(false), TCPConcurrent: ptr(true), UnifiedDelay: ptr(true), FindProcessMode: ptr("strict"), KeepAliveInterval: ptr(42), Sniffer: ptr(true)}
	if _, err = p.SetCoreSettings(want); err != nil {
		t.Fatal(err)
	}
	got, err := NewProfiles(p.dir).CoreSettings()
	if err != nil || got.LogLevel == nil || *got.LogLevel != "debug" || got.KeepAliveInterval == nil || *got.KeepAliveInterval != 42 {
		t.Fatalf("persist: %+v %v", got, err)
	}
	for _, bad := range []CoreOptions{{LogLevel: ptr("nope")}, {FindProcessMode: ptr("none")}, {KeepAliveInterval: ptr(0)}, {KeepAliveInterval: ptr(86401)}} {
		if _, err := p.SetCoreSettings(bad); err == nil {
			t.Fatalf("accepted invalid: %+v", bad)
		}
	}
	b, err := runtimeConfigWithOptions("ipv6: true\nsniffer:\n  enable: false\n  force-domain: [example.com]\nrules: [MATCH,DIRECT]\n", 7890, 9999, "secret", "", LANSettings{}, false, want)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err = yaml.Unmarshal(b, &cfg); err != nil {
		t.Fatal(err)
	}
	sniffer := cfg["sniffer"].(map[string]any)
	if cfg["ipv6"] != false || cfg["log-level"] != "debug" || sniffer["enable"] != true || sniffer["force-domain"] == nil {
		t.Fatalf("merge failed: %v", cfg)
	}
	b, err = runtimeConfigWithOptions("ipv6: true\nrules: [MATCH,DIRECT]\n", 7890, 9999, "secret", "", LANSettings{}, false, CoreOptions{})
	if err != nil || !strings.Contains(string(b), "ipv6: true") {
		t.Fatalf("inheritance: %s %v", b, err)
	}
}
func TestCoreOptionsRunningApplyAndReload(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO")
	}
	s := newTestService(t)
	defer s.Shutdown()
	p := NewProfilesWithService(t.TempDir(), s)
	if err := p.StartDraft(direct, freePort(t)); err != nil {
		t.Fatal(err)
	}
	want := CoreOptions{LogLevel: ptr("warning"), IPv6: ptr(false), TCPConcurrent: ptr(true), UnifiedDelay: ptr(true), FindProcessMode: ptr("strict"), KeepAliveInterval: ptr(30)}
	if _, err := p.SetCoreSettings(want); err != nil {
		t.Fatal(err)
	}
	check := func() {
		t.Helper()
		s.mu.Lock()
		defer s.mu.Unlock()
		var got struct {
			LogLevel string `json:"log-level"`
			IPv6     bool   `json:"ipv6"`
			TCP      bool   `json:"tcp-concurrent"`
			Unified  bool   `json:"unified-delay"`
		}
		if err := s.get("/configs", &got); err != nil || got.LogLevel != "warning" || got.IPv6 || !got.TCP || !got.Unified {
			t.Fatalf("effective settings: %+v %v", got, err)
		}
	}
	check()
	if running, err := s.reloadIfRunning(direct); err != nil || !running {
		t.Fatalf("reload: %v %v", running, err)
	}
	check()
	if _, err := p.SetCoreSettings(CoreOptions{LogLevel: ptr("bad")}); err == nil {
		t.Fatal("invalid setting accepted")
	}
	check()
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := p.StartDraft(direct, freePort(t)); err != nil {
		t.Fatal(err)
	}
	check()
}

func TestNoOpSaveDoesNotReloadCore(t *testing.T) {
	puts := 0
	s := metadataService(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/configs":
			_, _ = w.Write([]byte(`{"mode":"rule"}`))
		case r.Method == "GET" && r.URL.Path == "/proxies":
			_, _ = w.Write([]byte(`{"proxies":{}}`))
		case r.Method == "PUT" && r.URL.Path == "/configs":
			puts++
			w.WriteHeader(204)
		}
	})
	s.currentContent = "rules:\n  - MATCH,DIRECT\n"
	s.mixedPort = 7890
	s.secret = "s"
	s.coreOptions = CoreOptions{LogLevel: ptr("warning")}

	if ok, err := s.applyCoreOptions(CoreOptions{LogLevel: ptr("warning")}); err != nil || !ok {
		t.Fatalf("applyCoreOptions: %v %v", ok, err)
	}
	if puts != 0 {
		t.Fatalf("no-op applyCoreOptions triggered %d PUT /configs?force=true calls", puts)
	}

	if ok, err := s.reloadIfRunning(s.currentContent); err != nil || !ok {
		t.Fatalf("reloadIfRunning: %v %v", ok, err)
	}
	if puts != 0 {
		t.Fatalf("no-op reloadIfRunning triggered %d PUT /configs?force=true calls", puts)
	}

	if err := s.commitConfiguration(s.currentContent, func() error { return nil }); err != nil {
		t.Fatalf("commitConfiguration: %v", err)
	}
	if puts != 0 {
		t.Fatalf("no-op commitConfiguration triggered %d PUT /configs?force=true calls", puts)
	}
}
