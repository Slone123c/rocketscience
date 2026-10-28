package core

import (
	"os"
	"testing"
	"time"
)

func TestTrafficStream(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO to test a real core")
	}
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
