package main

import (
	"bytes"
	"errors"
	"image/png"
	"os"
	"rocketscience/internal/core"
	"rocketscience/internal/network"
	"strings"
	"testing"
	"unicode/utf8"
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
	saved := drawSpeeds
	drawSpeeds = nil
	defer func() { drawSpeeds = saved }()
	data := core.Telemetry{Connected: true, Up: 1024, Down: 1536}
	icon := makeTrayFrame("icon", true, data)
	if !icon.icon || icon.title != "" {
		t.Fatalf("icon: %+v", icon)
	}
	speed := makeTrayFrame("speed", true, data)
	if speed.icon || speed.title != "↑1.0K ↓1.5K" {
		t.Fatalf("speed: %+v", speed)
	}
	both := makeTrayFrame("both", true, data)
	if !both.icon || both.title != speed.title {
		t.Fatalf("both: %+v", both)
	}
	idle := makeTrayFrame("both", false, core.Telemetry{})
	if !idle.icon || idle.title != "" {
		t.Fatalf("idle both: %+v", idle)
	}
	stopped := makeTrayFrame("speed", false, core.Telemetry{})
	if stopped.title != "↑   — ↓   —" {
		t.Fatalf("stopped speed: %+v", stopped)
	}
	if speedText(-1) != "  0B" {
		t.Fatalf("negative: %q", speedText(-1))
	}
	for value, want := range map[int64]string{999: "999B", 1023: "1.0K", 10188: "9.9K", 10239: "10K", 1023999: "1.0M", 1048449024: "1.0G"} {
		got := speedText(value)
		if strings.TrimLeft(got, "\u2007") != want || utf8.RuneCountInString(got) != 4 {
			t.Fatalf("speedText(%d) = %q, want %q padded to four characters", value, got, want)
		}
	}
	tray := &testTraySurface{}
	if _, err := applyTrayFrame(tray, both, trayFrame{}, true, []byte{1}, []byte{2}); err != nil {
		t.Fatal(err)
	}
	if _, err := applyTrayFrame(tray, both, both, false, []byte{1}, []byte{2}); err != nil {
		t.Fatal(err)
	}
	if tray.icons != 1 || len(tray.titles) != 1 {
		t.Fatal("unchanged frame redrawn")
	}
	if _, err := applyTrayFrame(tray, speed, both, false, []byte{1}, []byte{2}); err != nil || tray.icons != 2 {
		t.Fatal("icon mode not updated")
	}
	tray.fail = true
	if _, err := applyTrayFrame(tray, both, speed, false, []byte{1}, []byte{2}); err == nil {
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

func TestTraySpeedsImage(t *testing.T) {
	speeds, ok := network.TraySpeeds("11K", "7.5K")
	if !ok {
		t.Skip("build the native helpers first")
	}
	text, err := png.Decode(bytes.NewReader(speeds))
	if err != nil {
		t.Fatal(err)
	}
	// Stacked lines keep the readout narrow: well under the old one-line title.
	if text.Bounds().Dy() != 36 || text.Bounds().Dx() > 64 {
		t.Fatalf("speeds image %v", text.Bounds())
	}
	wide, _ := network.TraySpeeds("888K", "1.0G")
	if w, _ := png.Decode(bytes.NewReader(wide)); w.Bounds().Dx() != text.Bounds().Dx() {
		t.Fatal("readout width changes with the value")
	}
	icon, err := trayAsset.ReadFile("assets/tray.png")
	if err != nil {
		t.Fatal(err)
	}
	tray := &testTraySurface{}
	frame := makeTrayFrame("both", true, core.Telemetry{Connected: true, Up: 11 * 1024, Down: 7680})
	drawn, err := applyTrayFrame(tray, frame, trayFrame{}, true, icon, transparentTrayIcon())
	if err != nil || !drawn || tray.icons != 1 || len(tray.titles) != 1 || tray.titles[0] != "" {
		t.Fatalf("drawn=%v err=%v icons=%d titles=%q", drawn, err, tray.icons, tray.titles)
	}
	frame.drawn = drawn
	if again, _ := applyTrayFrame(tray, frame, frame, false, icon, nil); !again || tray.icons != 1 {
		t.Fatal("unchanged readout redrawn")
	}
	S := os.Getenv("TRAY_PREVIEW")
	if S != "" {
		composed, _ := besideIcon(icon, speeds)
		_ = os.WriteFile(S, composed, 0o644)
	}
}
