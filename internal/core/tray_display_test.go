package core

import (
	"os/exec"
	"testing"
)

func TestTrayDisplayPreferenceAndTelemetry(t *testing.T) {
	p := NewProfiles(t.TempDir())
	value, err := p.TrayDisplay()
	if err != nil || value != "both" {
		t.Fatalf("default %q %v", value, err)
	}
	for _, mode := range []string{"icon", "speed", "both"} {
		if err := p.SetTrayDisplay(mode); err != nil {
			t.Fatal(err)
		}
		value, err = NewProfiles(p.dir).TrayDisplay()
		if err != nil || value != mode {
			t.Fatalf("saved %q %v", value, err)
		}
	}
	if err = p.SetTrayDisplay("unknown"); err == nil {
		t.Fatal("invalid mode accepted")
	}
	s := New()
	s.telemetry = Telemetry{Up: 1024, Down: 2048, Connected: true}
	if s.LiveTelemetry().Connected {
		t.Fatal("stopped core showed stale speed")
	}
	s.cmd = &exec.Cmd{}
	if got := s.LiveTelemetry(); !got.Connected || got.Up != 1024 || got.Down != 2048 {
		t.Fatalf("live: %+v", got)
	}
}
