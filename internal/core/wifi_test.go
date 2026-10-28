package core

import (
	"errors"
	"os"
	"rocketscience/internal/network"
	"testing"
	"time"
)

type fakeWiFiTarget struct {
	state wifiModeState
	fail  bool
	calls int
}

func (f *fakeWiFiTarget) wifiMode() (wifiModeState, error) { return f.state, nil }
func (f *fakeWiFiTarget) wifiSetMode(expected wifiModeState, mode string) (wifiModeState, error) {
	if f.fail {
		return wifiModeState{}, errors.New("mode failure")
	}
	if f.state != expected {
		return wifiModeState{}, errors.New("changed")
	}
	f.calls++
	f.state.mode = mode
	return f.state, nil
}
func wifiFixture(t *testing.T) (*WiFi, *fakeWiFiTarget, *network.Snapshot) {
	t.Helper()
	p := NewProfiles(t.TempDir())
	if _, err := p.SetWiFiSettings(WiFiSettings{Enabled: true, Trusted: []string{"Home", "Office"}}); err != nil {
		t.Fatal(err)
	}
	target := &fakeWiFiTarget{state: wifiModeState{running: true, mode: "global", revision: 1, runID: time.Unix(1, 0)}}
	snapshot := &network.Snapshot{Known: true, SSID: "Home"}
	return &WiFi{profiles: p, target: target, read: func() (network.Snapshot, error) { return *snapshot, nil }}, target, snapshot
}
func TestWiFiPolicyRestoresOnlyItsOwnMode(t *testing.T) {
	w, target, snapshot := wifiFixture(t)
	if err := w.Check(); err != nil || target.state.mode != "direct" || !w.State().Applied {
		t.Fatalf("trusted: %+v %v", w.State(), err)
	}
	snapshot.SSID = "Public"
	if err := w.Check(); err != nil || target.state.mode != "global" || w.State().Applied {
		t.Fatalf("leave: %+v %v", w.State(), err)
	}
	snapshot.SSID = "Home"
	if err := w.Check(); err != nil {
		t.Fatal(err)
	}
	target.state.revision++
	target.state.mode = "rule"
	if err := w.Check(); err != nil || target.state.mode != "rule" || !w.State().ManualOverride {
		t.Fatalf("manual: %+v %v", w.State(), err)
	}
	snapshot.SSID = "Public"
	if err := w.Check(); err != nil || target.state.mode != "rule" {
		t.Fatal("manual choice overwritten on leaving")
	}
	snapshot.SSID = "Office"
	if err := w.Check(); err != nil || target.state.mode != "direct" {
		t.Fatal("new trusted network not applied")
	}
	snapshot.Known = false
	snapshot.SSID = ""
	if err := w.Check(); err != nil || target.state.mode != "rule" {
		t.Fatal("unknown network retained automatic direct mode")
	}
}
func TestWiFiPolicyStopsRestartsAndFailures(t *testing.T) {
	w, target, snapshot := wifiFixture(t)
	target.fail = true
	if err := w.Check(); err == nil || w.State().Applied {
		t.Fatal("failed switch claimed ownership")
	}
	target.fail = false
	if err := w.Check(); err != nil {
		t.Fatal(err)
	}
	target.state.runID = time.Unix(2, 0) // internal restart keeps user intent revision
	if err := w.Check(); err != nil {
		t.Fatal(err)
	}
	snapshot.SSID = "Public"
	target.fail = true
	if err := w.Check(); err == nil || !w.State().Applied {
		t.Fatal("failed restore forgot ownership")
	}
	target.fail = false
	if err := w.Check(); err != nil || target.state.mode != "global" {
		t.Fatal("restore retry failed")
	}
	snapshot.SSID = "Home"
	target.state.running = false
	before := target.calls
	if err := w.Check(); err != nil || target.calls != before {
		t.Fatal("stopped core was changed")
	}
	target.state.running = true
	target.state.revision++
	target.state.runID = time.Unix(3, 0)
	if err := w.Check(); err != nil || target.state.mode != "direct" {
		t.Fatal("explicit restart did not apply trusted policy")
	}
	if _, err := w.SetSettings(WiFiSettings{Enabled: false, Trusted: []string{"Home"}}); err != nil || target.state.mode != "global" {
		t.Fatal("disabling feature did not restore mode")
	}
}
func TestWiFiAlreadyDirectAndPreferences(t *testing.T) {
	w, target, snapshot := wifiFixture(t)
	target.state.mode = "direct"
	if err := w.Check(); err != nil || w.State().Applied {
		t.Fatal("claimed an existing direct choice")
	}
	snapshot.SSID = "Public"
	if err := w.Check(); err != nil || target.state.mode != "direct" {
		t.Fatal("preexisting direct choice overwritten")
	}
	value, err := w.profiles.SetWiFiSettings(WiFiSettings{Enabled: true, Trusted: []string{" Home ", "home", " Home "}})
	if err != nil || len(value.Trusted) != 2 || value.Trusted[0] != " Home " {
		t.Fatalf("SSID identity changed %+v %v", value, err)
	}
	for _, name := range []string{"", "bad\nname", "123456789012345678901234567890123"} {
		if _, err := w.profiles.SetWiFiSettings(WiFiSettings{Trusted: []string{name}}); err == nil {
			t.Fatal("invalid SSID accepted")
		}
	}
}
func TestWiFiPolicyRealCoreModes(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO")
	}
	s := newTestService(t)
	s.geo.dir = t.TempDir()
	defer s.Shutdown()
	p := NewProfilesWithService(t.TempDir(), s)
	if err := s.Start(direct, freePort(t)); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMode("global"); err != nil {
		t.Fatal(err)
	}
	snapshot := network.Snapshot{Known: true, SSID: "Home"}
	w := NewWiFi(p, s, func() (network.Snapshot, error) { return snapshot, nil })
	if _, err := w.SetSettings(WiFiSettings{Enabled: true, Trusted: []string{"Home"}}); err != nil {
		t.Fatal(err)
	}
	if s.Status().Mode != "direct" {
		t.Fatal("trusted mode not applied")
	}
	snapshot.SSID = "Public"
	if err := w.Check(); err != nil || s.Status().Mode != "global" {
		t.Fatal("previous mode not restored")
	}
	snapshot.SSID = "Home"
	if err := w.Check(); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMode("rule"); err != nil {
		t.Fatal(err)
	}
	if err := w.Check(); err != nil || s.Status().Mode != "rule" || !w.State().ManualOverride {
		t.Fatal("manual mode not preserved")
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := w.Check(); err != nil || s.Status().Running {
		t.Fatal("policy restarted stopped core")
	}
}

func TestWiFiManualDirectChoiceAndNewStart(t *testing.T) {
	w, target, snapshot := wifiFixture(t)
	if err := w.Check(); err != nil {
		t.Fatal(err)
	}
	target.state.revision++ // choosing direct manually also cancels automatic ownership
	if err := w.Check(); err != nil || w.State().Applied || !w.State().ManualOverride {
		t.Fatalf("manual direct: %+v %v", w.State(), err)
	}
	snapshot.SSID = "Public"
	if err := w.Check(); err != nil || target.state.mode != "direct" {
		t.Fatal("manual direct choice restored unexpectedly")
	}
	snapshot.SSID = "Home"
	target.state.mode = "rule"
	target.state.revision++
	target.state.runID = time.Unix(4, 0)
	if err := w.Check(); err != nil || target.state.mode != "direct" {
		t.Fatal("new core start ignored trusted network")
	}
	snapshot.SSID = "Public"
	if err := w.Check(); err != nil || target.state.mode != "rule" {
		t.Fatal("new start mode not restored")
	}
}
