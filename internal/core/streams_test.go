package core

import (
	"path/filepath"
	"testing"
	"time"
)

func TestTrafficStream(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", filepath.Join(t.TempDir(), "missing"))
	s := newTestService(t)
	defer s.Shutdown()
	if err := s.Start(direct, freePort(t)); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if v := s.Streams(); v.Telemetry.Connected && v.Memory.Connected && len(v.History.Up) > 0 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("traffic stream did not connect")
}
