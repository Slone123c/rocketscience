package main

import (
	"errors"
	"rocketscience/internal/core"
	"testing"
)

type testTraySurface struct {
	icons  int
	titles []string
	fail   bool
}

func (t *testTraySurface) SetIcon(_ []byte, _ bool) error {
	t.icons++
	if t.fail {
		return errors.New("icon failure")
	}
	return nil
}
func (t *testTraySurface) SetTitle(value string) { t.titles = append(t.titles, value) }
func TestTrayDisplayFrames(t *testing.T) {
	data := core.Telemetry{Connected: true, Up: 1024, Down: 1536}
	icon := makeTrayFrame("icon", data)
	if !icon.icon || icon.title != "" {
		t.Fatalf("icon: %+v", icon)
	}
	speed := makeTrayFrame("speed", data)
	if speed.icon || speed.title != "↑ 1.0K ↓ 1.5K" {
		t.Fatalf("speed: %+v", speed)
	}
	both := makeTrayFrame("both", data)
	if !both.icon || both.title != speed.title {
		t.Fatalf("both: %+v", both)
	}
	idle := makeTrayFrame("both", core.Telemetry{})
	if idle.title != "↑   — ↓   —" {
		t.Fatalf("idle: %+v", idle)
	}
	if speedText(-1) != "   0B" {
		t.Fatalf("negative: %q", speedText(-1))
	}
	tray := &testTraySurface{}
	if err := applyTrayFrame(tray, both, trayFrame{}, true, []byte{1}, []byte{2}); err != nil {
		t.Fatal(err)
	}
	if err := applyTrayFrame(tray, both, both, false, []byte{1}, []byte{2}); err != nil {
		t.Fatal(err)
	}
	if tray.icons != 1 || len(tray.titles) != 1 {
		t.Fatal("unchanged frame redrawn")
	}
	if err := applyTrayFrame(tray, speed, both, false, []byte{1}, []byte{2}); err != nil || tray.icons != 2 {
		t.Fatal("icon mode not updated")
	}
	tray.fail = true
	if err := applyTrayFrame(tray, both, speed, false, []byte{1}, []byte{2}); err == nil {
		t.Fatal("icon error ignored")
	}
}
func TestTrayDisplaySettingBridge(t *testing.T) {
	p := core.NewProfiles(t.TempDir())
	d := &Desktop{profiles: p, trayWake: make(chan struct{}, 1)}
	if err := d.SetTrayDisplay("speed"); err != nil {
		t.Fatal(err)
	}
	if value, err := d.TrayDisplay(); err != nil || value != "speed" {
		t.Fatalf("saved: %s %v", value, err)
	}
	if d.display != "speed" || len(d.trayWake) != 1 {
		t.Fatal("menu was not notified")
	}
	if err := d.SetTrayDisplay("bad"); err == nil {
		t.Fatal("invalid mode accepted")
	}
	if d.display != "speed" {
		t.Fatal("failed update changed display")
	}
}
