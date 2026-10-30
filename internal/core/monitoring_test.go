package core

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func waitMonitoring(t *testing.T, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(7 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(message)
}

func startTestMonitoring(t *testing.T, s *Service) (func(), uint64, <-chan struct{}) {
	t.Helper()
	done := make(chan struct{})
	var once sync.Once
	stop := func() { once.Do(func() { close(done) }) }
	s.startStreamsAt("", "", done)
	s.streamMu.Lock()
	generation := s.streamGeneration
	finished := s.streamFinished
	s.streamMu.Unlock()
	t.Cleanup(func() {
		stop()
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
			t.Error("monitoring workers did not stop")
		}
	})
	return stop, generation, finished
}

func TestMonitoringBuffersAndStop(t *testing.T) {
	s := New()
	stop, gen, finished := startTestMonitoring(t, s)
	s.streamMu.Lock()
	s.trafficNext, s.trafficCount = 0, 0
	s.logs = nil
	s.streamMu.Unlock()
	for i := int64(1); i <= 30; i++ {
		s.recordTrafficSample(gen, i, i*2)
	}
	s.recordMemorySample(gen, 64*1024*1024)
	for i := 0; i < 510; i++ {
		s.recordLogEntry(gen, LogEntry{Type: "debug", Payload: fmt.Sprint(i)})
	}
	v := s.Streams()
	if len(v.History.Up) != 24 || v.History.Up[0] != 7 || v.History.Down[23] != 60 || v.Memory.InUse != 64*1024*1024 || len(v.Logs) != 500 || v.Logs[0].Payload != "10" || v.Logs[499].Payload != "509" || v.Logs[0].Time == "" {
		t.Fatalf("bad snapshot: %+v", v)
	}
	v.History.Up[0] = -1
	v.Logs[0].Payload = "mutated"
	if next := s.Streams(); next.History.Up[0] != 7 || next.Logs[0].Payload != "10" {
		t.Fatal("caller mutated monitoring storage")
	}
	stop()
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("streams blocked on idle reads after stop")
	}
	if next := s.Streams(); next.Telemetry.Connected || next.Memory.Connected || len(next.History.Up) != 0 {
		t.Fatal("stopped streams still report connected")
	}
}

func TestMonitoringReplacementCancelsPreviousSession(t *testing.T) {
	s := New()
	stopOld, oldGen, oldFinished := startTestMonitoring(t, s)
	s.recordTrafficSample(oldGen, 10, 10)
	waitMonitoring(t, func() bool { return s.Streams().Telemetry.Up == 10 }, "old session missing")
	_, newGen, _ := startTestMonitoring(t, s)
	select {
	case <-oldFinished:
	case <-time.After(3 * time.Second):
		t.Fatal("replacement leaked previous session")
	}
	s.streamMu.Lock()
	s.trafficNext, s.trafficCount = 0, 0
	s.streamMu.Unlock()
	s.recordTrafficSample(newGen, 100, 100)
	s.recordTrafficSample(oldGen, 999, 999)
	stopOld()
	if v := s.Streams(); !v.Telemetry.Connected || len(v.History.Up) != 1 || v.History.Up[0] != 100 {
		t.Fatal("old session contaminated replacement state")
	}
}

func TestMonitoringRejectsInvalidSamples(t *testing.T) {
	s := New()
	_, gen, _ := startTestMonitoring(t, s)
	s.streamMu.Lock()
	s.telemetry, s.memory = Telemetry{}, MemoryUsage{}
	s.trafficNext, s.trafficCount = 0, 0
	s.streamMu.Unlock()
	s.recordTrafficSample(gen, -1, 2)
	s.recordMemorySample(gen, -5)
	if v := s.Streams(); v.Telemetry.Connected || v.Memory.Connected || len(v.History.Up) != 0 {
		t.Fatal("invalid samples displayed as valid")
	}
}
