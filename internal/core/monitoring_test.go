package core

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/net/websocket"
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

func startTestMonitoring(t *testing.T, s *Service, server *httptest.Server) (func(), <-chan struct{}) {
	t.Helper()
	done := make(chan struct{})
	var once sync.Once
	stop := func() { once.Do(func() { close(done) }) }
	s.startStreamsAt(strings.TrimPrefix(server.URL, "http://"), "test +?&", done)
	s.streamMu.Lock()
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
	return stop, finished
}

func TestMonitoringBuffersAndStop(t *testing.T) {
	var closed atomic.Int32
	server := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		defer closed.Add(1)
		if ws.Request().URL.Query().Get("token") != "test +?&" {
			t.Error("stream token corrupted")
		}
		switch ws.Request().URL.Path {
		case "/traffic":
			for i := int64(1); i <= 30; i++ {
				_ = websocket.JSON.Send(ws, map[string]int64{"up": i, "down": i * 2})
			}
		case "/memory":
			_ = websocket.JSON.Send(ws, map[string]int64{"inuse": 64 * 1024 * 1024})
		case "/logs":
			if ws.Request().URL.Query().Get("level") != "debug" {
				t.Error("debug logs omitted")
			}
			for i := 0; i < 510; i++ {
				_ = websocket.JSON.Send(ws, LogEntry{Type: "debug", Payload: fmt.Sprint(i)})
			}
		default:
			t.Error("unexpected stream")
		}
		var ignored any
		_ = websocket.JSON.Receive(ws, &ignored)
	}))
	defer server.Close()
	s := New()
	stop, finished := startTestMonitoring(t, s, server)
	waitMonitoring(t, func() bool {
		v := s.Streams()
		return v.Telemetry.Up == 30 && v.Memory.Connected && len(v.Logs) == 500 && v.Logs[499].Payload == "509"
	}, "monitoring did not receive complete buffers")
	v := s.Streams()
	if len(v.History.Up) != 24 || v.History.Up[0] != 7 || v.History.Down[23] != 60 || v.Memory.InUse != 64*1024*1024 || v.Logs[0].Payload != "10" || v.Logs[0].Time == "" {
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
	waitMonitoring(t, func() bool { return closed.Load() == 3 }, "stream sockets not closed")
	if next := s.Streams(); next.Telemetry.Connected || next.Memory.Connected || len(next.History.Up) != 0 {
		t.Fatal("stopped streams still report connected")
	}
}

func TestMonitoringDisconnectReconnect(t *testing.T) {
	drop := make(chan struct{})
	var dropOnce sync.Once
	closeDrop := func() { dropOnce.Do(func() { close(drop) }) }
	t.Cleanup(closeDrop)
	var calls atomic.Int32
	server := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		switch ws.Request().URL.Path {
		case "/traffic":
			n := calls.Add(1)
			_ = websocket.JSON.Send(ws, map[string]int64{"up": int64(n) * 100, "down": 0})
			if n == 1 {
				<-drop
				return
			}
		case "/memory":
			_ = websocket.JSON.Send(ws, map[string]int64{"inuse": 0})
		}
		var ignored any
		_ = websocket.JSON.Receive(ws, &ignored)
	}))
	defer server.Close()
	s := New()
	startTestMonitoring(t, s, server)
	waitMonitoring(t, func() bool { return s.Streams().Telemetry.Connected }, "initial stream failed")
	closeDrop()
	waitMonitoring(t, func() bool { return !s.Streams().Telemetry.Connected }, "disconnect retained old speed")
	if len(s.Streams().History.Up) != 0 {
		t.Fatal("disconnect retained continuous chart")
	}
	waitMonitoring(t, func() bool { return s.Streams().Telemetry.Up == 200 && s.Streams().Telemetry.Connected }, "stream did not reconnect")
	if v := s.Streams(); len(v.History.Up) != 1 || v.History.Up[0] != 200 {
		t.Fatal("reconnected history contains stale samples")
	}
}

func TestMonitoringStopDuringHandshake(t *testing.T) {
	started := make(chan struct{}, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		<-r.Context().Done()
	}))
	defer server.Close()
	s := New()
	stop, finished := startTestMonitoring(t, s, server)
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("handshake did not start")
	}
	stop()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("stop did not cancel pending handshake")
	}
}

func TestMonitoringStaleSamplesExpire(t *testing.T) {
	server := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		switch ws.Request().URL.Path {
		case "/traffic":
			_ = websocket.JSON.Send(ws, map[string]int64{"up": 1, "down": 2})
		case "/memory":
			_ = websocket.JSON.Send(ws, map[string]int64{"inuse": 100})
		}
		var ignored any
		_ = websocket.JSON.Receive(ws, &ignored)
	}))
	defer server.Close()
	s := New()
	startTestMonitoring(t, s, server)
	waitMonitoring(t, func() bool { v := s.Streams(); return v.Telemetry.Connected && v.Memory.Connected }, "initial samples missing")
	waitMonitoring(t, func() bool { v := s.Streams(); return !v.Telemetry.Connected && !v.Memory.Connected }, "stale samples remained connected")
}

func TestMonitoringReplacementCancelsPreviousSession(t *testing.T) {
	makeServer := func(value int64) *httptest.Server {
		return httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
			defer ws.Close()
			if ws.Request().URL.Path == "/traffic" {
				_ = websocket.JSON.Send(ws, map[string]int64{"up": value, "down": value})
			}
			var ignored any
			_ = websocket.JSON.Receive(ws, &ignored)
		}))
	}
	oldServer, newServer := makeServer(10), makeServer(100)
	defer oldServer.Close()
	defer newServer.Close()
	s := New()
	stopOld, oldFinished := startTestMonitoring(t, s, oldServer)
	waitMonitoring(t, func() bool { return s.Streams().Telemetry.Up == 10 }, "old session missing")
	startTestMonitoring(t, s, newServer)
	select {
	case <-oldFinished:
	case <-time.After(3 * time.Second):
		t.Fatal("replacement leaked previous session")
	}
	waitMonitoring(t, func() bool { return s.Streams().Telemetry.Up == 100 }, "replacement samples missing")
	stopOld()
	if v := s.Streams(); !v.Telemetry.Connected || len(v.History.Up) != 1 || v.History.Up[0] != 100 {
		t.Fatal("old session contaminated replacement state")
	}
}

func TestMonitoringRejectsInvalidSamples(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		switch ws.Request().URL.Path {
		case "/traffic":
			calls.Add(1)
			_ = websocket.JSON.Send(ws, map[string]int64{"up": -1, "down": 2})
		case "/memory":
			calls.Add(1)
			_ = websocket.JSON.Send(ws, map[string]int64{"other": 123})
		}
		var ignored any
		_ = websocket.JSON.Receive(ws, &ignored)
	}))
	defer server.Close()
	s := New()
	startTestMonitoring(t, s, server)
	waitMonitoring(t, func() bool { return calls.Load() >= 4 }, "invalid stream was not retried")
	if v := s.Streams(); v.Telemetry.Connected || v.Memory.Connected || len(v.History.Up) != 0 {
		t.Fatal("invalid samples displayed as valid")
	}
}
