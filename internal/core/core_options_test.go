package core

import (
	"runtime"
	"strings"
	"testing"

	"github.com/metacubex/mihomo/hub/executor"
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
	t.Setenv("ROCKETSCIENCE_MIHOMO", t.TempDir()+"/missing")
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
		gen := executor.GetGeneral()
		if gen.LogLevel.String() != "warning" || gen.IPv6 || !gen.TCPConcurrent || !gen.UnifiedDelay {
			t.Fatalf("effective settings: %+v", gen)
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
	t.Setenv("ROCKETSCIENCE_MIHOMO", t.TempDir()+"/missing")
	s := newTestService(t)
	defer s.Shutdown()
	if err := s.Start(direct, freePort(t)); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.applyCoreOptions(CoreOptions{LogLevel: ptr("warning")}); err != nil || !ok {
		t.Fatalf("applyCoreOptions initial: %v %v", ok, err)
	}
	// Make runtimeDir unsearchable so any actual applyRuntimeConfigLocked write would fail,
	// proving that no-op applyCoreOptions / reloadIfRunning / commitConfiguration skip reloading.
	s.mu.Lock()
	realDir := s.runtimeDir
	s.runtimeDir = t.TempDir() + "/missing/sub"
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.runtimeDir = realDir
		s.mu.Unlock()
	}()

	if ok, err := s.applyCoreOptions(CoreOptions{LogLevel: ptr("warning")}); err != nil || !ok {
		t.Fatalf("applyCoreOptions: %v %v", ok, err)
	}
	if ok, err := s.reloadIfRunning(s.currentContent); err != nil || !ok {
		t.Fatalf("reloadIfRunning: %v %v", ok, err)
	}
	if err := s.commitConfiguration(s.currentContent, func() error { return nil }); err != nil {
		t.Fatalf("commitConfiguration: %v", err)
	}
}

func TestConcurrentMultiSettingsPreserveAllFields(t *testing.T) {
	withTestGeoHome(t)
	p := NewProfiles(t.TempDir())
	fns := []func() error{
		func() error { return p.SetPort(47892) },
		func() error { _, err := p.SetLANSettings(LANSettings{Enabled: true, Address: "*"}); return err },
		func() error {
			_, err := p.SetProxyBypass(ProxyBypassSettings{Custom: true, Entries: []string{"*.corp.example"}})
			return err
		},
		func() error { _, err := p.SetTunSettings(TunOptions{Stack: ptr("gvisor"), MTU: ptr(1400)}); return err },
		func() error {
			_, err := p.SetDNSSettings(DNSOptions{Enabled: ptr(true), EnhancedMode: ptr("fake-ip")})
			return err
		},
		func() error {
			_, err := p.SetCoreSettings(CoreOptions{LogLevel: ptr("warning"), TCPConcurrent: ptr(true)})
			return err
		},
		func() error {
			_, err := p.SetInboundSettings(InboundOptions{HTTPPort: 47893, SOCKSPort: 47894})
			return err
		},
		func() error {
			_, err := p.SetControllerSettings(ControllerOptions{Address: "127.0.0.1", Port: 47895})
			return err
		},
		func() error {
			_, err := p.SetGeoSettings(GeoOptions{DataMode: ptr(true), AutoUpdate: ptr(true), UpdateInterval: ptr(48)})
			return err
		},
		func() error {
			_, err := p.SetSpeedSettings(SpeedSettings{URL: "https://cp.cloudflare.com/generate_204", TimeoutMS: 2500})
			return err
		},
		func() error {
			_, err := p.SetWiFiSettings(WiFiSettings{Enabled: true, Trusted: []string{"CorpWiFi"}})
			return err
		},
		func() error { return p.SetTrayDisplay("speed") },
		func() error { return p.SetHideDockWhenClosed(true) },
	}
	errs := make(chan error, len(fns))
	for _, fn := range fns {
		fn := fn
		go func() { errs <- fn() }()
	}
	for range fns {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent setting save failed: %v", err)
		}
	}
	reloaded := NewProfiles(p.dir)
	st, err := reloaded.State()
	if err != nil || st.MixedPort != 47892 {
		t.Fatalf("MixedPort lost: %+v %v", st, err)
	}
	lan, err := reloaded.LANSettings()
	if err != nil || !lan.Enabled || lan.Address != "*" {
		t.Fatalf("LAN lost: %+v %v", lan, err)
	}
	bypass, err := reloaded.ProxyBypass()
	if err != nil || !bypass.Custom || len(bypass.Entries) != 1 || bypass.Entries[0] != "*.corp.example" {
		t.Fatalf("ProxyBypass lost: %+v %v", bypass, err)
	}
	tunOpt, err := reloaded.TunSettings()
	if err != nil || tunOpt.Stack == nil || *tunOpt.Stack != "gvisor" || tunOpt.MTU == nil || *tunOpt.MTU != 1400 {
		t.Fatalf("TunSettings lost: %+v %v", tunOpt, err)
	}
	dnsOpt, err := reloaded.DNSSettings()
	if err != nil || dnsOpt.Enabled == nil || !*dnsOpt.Enabled || dnsOpt.EnhancedMode == nil || *dnsOpt.EnhancedMode != "fake-ip" {
		t.Fatalf("DNSSettings lost: %+v %v", dnsOpt, err)
	}
	coreOpt, err := reloaded.CoreSettings()
	if err != nil || coreOpt.LogLevel == nil || *coreOpt.LogLevel != "warning" || coreOpt.TCPConcurrent == nil || !*coreOpt.TCPConcurrent {
		t.Fatalf("CoreSettings lost: %+v %v", coreOpt, err)
	}
	inbOpt, err := reloaded.InboundSettings()
	if err != nil || inbOpt.HTTPPort != 47893 || inbOpt.SOCKSPort != 47894 {
		t.Fatalf("InboundSettings lost: %+v %v", inbOpt, err)
	}
	ctrlOpt, err := reloaded.ControllerSettings()
	if err != nil || ctrlOpt.Port != 47895 {
		t.Fatalf("ControllerSettings lost: %+v %v", ctrlOpt, err)
	}
	geoOpt, err := reloaded.GeoSettings()
	if err != nil || geoOpt.DataMode == nil || !*geoOpt.DataMode || geoOpt.UpdateInterval == nil || *geoOpt.UpdateInterval != 48 {
		t.Fatalf("GeoSettings lost: %+v %v", geoOpt, err)
	}
	speedOpt, err := reloaded.SpeedSettings()
	if err != nil || speedOpt.TimeoutMS != 2500 {
		t.Fatalf("SpeedSettings lost: %+v %v", speedOpt, err)
	}
	wifiOpt, err := reloaded.WiFiSettings()
	if err != nil || !wifiOpt.Enabled || len(wifiOpt.Trusted) != 1 || wifiOpt.Trusted[0] != "CorpWiFi" {
		t.Fatalf("WiFiSettings lost: %+v %v", wifiOpt, err)
	}
	trayMode, err := reloaded.TrayDisplay()
	if err != nil || trayMode != "speed" {
		t.Fatalf("TrayDisplay lost: %s %v", trayMode, err)
	}
	hideDock, err := reloaded.HideDockWhenClosed()
	wantHideDock := runtime.GOOS == "darwin"
	if err != nil || hideDock != wantHideDock {
		t.Fatalf("HideDockWhenClosed: got %v want %v (err=%v)", hideDock, wantHideDock, err)
	}
}
