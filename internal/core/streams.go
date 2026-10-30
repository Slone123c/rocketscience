package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	mihomoLog "github.com/metacubex/mihomo/log"
	"github.com/metacubex/mihomo/tunnel/statistic"
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

func (s *Service) startStreamsAt(_, _ string, done <-chan struct{}) {
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
	up, down := statistic.DefaultManager.Now()
	s.recordTrafficSample(generation, up, down)
	s.recordMemorySample(generation, int64(statistic.DefaultManager.Memory()))

	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		s.runMetricsStream(ctx, generation)
	}()
	go func() {
		defer workers.Done()
		s.runLogStream(ctx, generation)
	}()
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

func (s *Service) runMetricsStream(ctx context.Context, generation uint64) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			up, down := statistic.DefaultManager.Now()
			s.recordTrafficSample(generation, up, down)
			s.recordMemorySample(generation, int64(statistic.DefaultManager.Memory()))
		}
	}
}

func (s *Service) runLogStream(ctx context.Context, generation uint64) {
	sub := mihomoLog.Subscribe()
	defer mihomoLog.UnSubscribe(sub)
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-sub:
			if !ok {
				return
			}
			if ev.LogLevel < mihomoLog.DEBUG {
				continue
			}
			s.recordLogEntry(generation, LogEntry{
				Type:    ev.Type(),
				Payload: ev.Payload,
			})
		}
	}
}

func (s *Service) recordTrafficSample(generation uint64, up, down int64) {
	if up < 0 || down < 0 {
		return
	}
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	if s.streamGeneration == generation {
		s.telemetry = Telemetry{Up: up, Down: down, Connected: true}
		s.trafficSamples[s.trafficNext] = trafficSample{up, down}
		s.trafficNext = (s.trafficNext + 1) % len(s.trafficSamples)
		if s.trafficCount < len(s.trafficSamples) {
			s.trafficCount++
		}
	}
}

func (s *Service) recordMemorySample(generation uint64, inuse int64) {
	if inuse < 0 {
		return
	}
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	if s.streamGeneration == generation {
		s.memory = MemoryUsage{InUse: inuse, Connected: true}
	}
}

func (s *Service) recordLogEntry(generation uint64, entry LogEntry) {
	if len(entry.Payload) > 4096 {
		entry.Payload = entry.Payload[:4096] + "…"
	}
	entry.Time = time.Now().Format(time.RFC3339Nano)
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	if s.streamGeneration == generation {
		if len(s.logs) == 500 {
			copy(s.logs, s.logs[1:])
			s.logs[499] = entry
		} else {
			s.logs = append(s.logs, entry)
		}
	}
}
