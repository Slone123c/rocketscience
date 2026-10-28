package core

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

type Telemetry struct {
	Up        int64 `json:"up"`
	Down      int64 `json:"down"`
	Connected bool  `json:"connected"`
}
type MemoryUsage struct {
	InUse     int64 `json:"inuse"`
	Connected bool  `json:"connected"`
}
type TrafficHistory struct {
	Up   []int64 `json:"up"`
	Down []int64 `json:"down"`
}
type trafficSample struct{ up, down int64 }
type LogEntry struct {
	Time    string `json:"time"`
	Type    string `json:"type"`
	Payload string `json:"payload"`
}
type Streams struct {
	Telemetry Telemetry      `json:"telemetry"`
	Memory    MemoryUsage    `json:"memory"`
	History   TrafficHistory `json:"history"`
	Logs      []LogEntry     `json:"logs"`
}

// Monitoring is the small snapshot used outside the log page.
type Monitoring struct {
	Telemetry Telemetry      `json:"telemetry"`
	Memory    MemoryUsage    `json:"memory"`
	History   TrafficHistory `json:"history"`
}

func (s *Service) Monitoring() Monitoring {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	return Monitoring{Telemetry: s.telemetry, Memory: s.memory, History: s.trafficHistory()}
}

// Called with streamMu held. Every returned slice belongs to the caller.
func (s *Service) trafficHistory() TrafficHistory {
	history := TrafficHistory{Up: make([]int64, 0, s.trafficCount), Down: make([]int64, 0, s.trafficCount)}
	for i := 0; i < s.trafficCount; i++ {
		sample := s.trafficSamples[(s.trafficNext-s.trafficCount+i+len(s.trafficSamples))%len(s.trafficSamples)]
		history.Up = append(history.Up, sample.up)
		history.Down = append(history.Down, sample.down)
	}
	return history
}

func (s *Service) Streams() Streams {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	return Streams{Telemetry: s.telemetry, Memory: s.memory, History: s.trafficHistory(), Logs: append([]LogEntry{}, s.logs...)}
}

// LiveTelemetry avoids copying history and logs for native menu-bar updates.
func (s *Service) LiveTelemetry() Telemetry {
	s.mu.Lock()
	running := s.cmd != nil
	s.mu.Unlock()
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	value := s.telemetry
	if !running {
		value.Connected = false
	}
	return value
}
func (s *Service) startStreams(port int, secret string, done <-chan struct{}) {
	s.startStreamsAt(fmt.Sprintf("127.0.0.1:%d", port), secret, done)
}

func (s *Service) startStreamsAt(authority, secret string, done <-chan struct{}) {
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	s.streamMu.Lock()
	previousCancel := s.streamCancel
	s.streamGeneration++
	generation := s.streamGeneration
	s.streamCancel, s.streamFinished = cancel, finished
	s.telemetry, s.memory = Telemetry{}, MemoryUsage{}
	s.trafficNext, s.trafficCount = 0, 0
	s.logs = nil
	s.streamMu.Unlock()
	if previousCancel != nil {
		previousCancel()
	}
	var workers sync.WaitGroup
	for _, kind := range []string{"traffic", "memory", "logs"} {
		workers.Add(1)
		go func(kind string) { defer workers.Done(); s.runStream(ctx, generation, authority, secret, kind) }(kind)
	}
	go func() {
		select {
		case <-done:
		case <-ctx.Done():
		}
		cancel()
		workers.Wait()
		s.streamMu.Lock()
		if s.streamGeneration == generation {
			s.telemetry.Connected, s.memory.Connected = false, false
			s.trafficCount, s.trafficNext = 0, 0
		}
		s.streamMu.Unlock()
		close(finished)
	}()
}

func (s *Service) runStream(ctx context.Context, generation uint64, authority, secret, kind string) {
	for ctx.Err() == nil {
		query := url.Values{"token": {secret}}
		if kind == "logs" {
			query.Set("level", "debug")
		}
		address := fmt.Sprintf("ws://%s/%s?%s", authority, kind, query.Encode())
		config, err := websocket.NewConfig(address, "http://127.0.0.1/")
		if err != nil {
			return
		}
		dialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		ws, err := config.DialContext(dialCtx)
		cancel()
		if err == nil {
			s.receiveStream(ctx, generation, kind, ws)
			_ = ws.Close()
		}
		s.streamMu.Lock()
		if s.streamGeneration == generation {
			switch kind {
			case "traffic":
				s.telemetry.Connected = false
				s.trafficCount, s.trafficNext = 0, 0
			case "memory":
				s.memory.Connected = false
			}
		}
		s.streamMu.Unlock()
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (s *Service) receiveStream(ctx context.Context, generation uint64, kind string, ws *websocket.Conn) {
	ws.MaxPayloadBytes = 1 << 20
	received := make(chan struct{})
	defer close(received)
	go func() {
		select {
		case <-ctx.Done():
			_ = ws.Close()
		case <-received:
		}
	}()
	for ctx.Err() == nil {
		if kind != "logs" {
			_ = ws.SetReadDeadline(time.Now().Add(4 * time.Second))
		}
		switch kind {
		case "traffic":
			var sample struct {
				Up   *int64 `json:"up"`
				Down *int64 `json:"down"`
			}
			if websocket.JSON.Receive(ws, &sample) != nil || sample.Up == nil || sample.Down == nil || *sample.Up < 0 || *sample.Down < 0 {
				return
			}
			s.streamMu.Lock()
			if s.streamGeneration == generation && ctx.Err() == nil {
				s.telemetry = Telemetry{Up: *sample.Up, Down: *sample.Down, Connected: true}
				s.trafficSamples[s.trafficNext] = trafficSample{*sample.Up, *sample.Down}
				s.trafficNext = (s.trafficNext + 1) % len(s.trafficSamples)
				if s.trafficCount < len(s.trafficSamples) {
					s.trafficCount++
				}
			}
			s.streamMu.Unlock()
		case "memory":
			var sample struct {
				InUse *int64 `json:"inuse"`
			}
			if websocket.JSON.Receive(ws, &sample) != nil || sample.InUse == nil || *sample.InUse < 0 {
				return
			}
			s.streamMu.Lock()
			if s.streamGeneration == generation && ctx.Err() == nil {
				s.memory = MemoryUsage{InUse: *sample.InUse, Connected: true}
			}
			s.streamMu.Unlock()
		case "logs":
			var entry LogEntry
			if websocket.JSON.Receive(ws, &entry) != nil {
				return
			}
			entry.Time = time.Now().Format(time.RFC3339Nano)
			s.streamMu.Lock()
			if s.streamGeneration == generation && ctx.Err() == nil {
				if len(s.logs) == 500 {
					copy(s.logs, s.logs[1:])
					s.logs[499] = entry
				} else {
					s.logs = append(s.logs, entry)
				}
			}
			s.streamMu.Unlock()
		}
	}
}
