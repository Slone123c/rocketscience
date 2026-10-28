package core

import (
	"fmt"
	"net/url"
	"time"

	"golang.org/x/net/websocket"
)

type Telemetry struct {
	Up        int64 `json:"up"`
	Down      int64 `json:"down"`
	Connected bool  `json:"connected"`
}
type LogEntry struct {
	Time    string `json:"time"`
	Type    string `json:"type"`
	Payload string `json:"payload"`
}
type Streams struct {
	Telemetry Telemetry  `json:"telemetry"`
	Logs      []LogEntry `json:"logs"`
}

func (s *Service) Streams() Streams {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	return Streams{Telemetry: s.telemetry, Logs: append([]LogEntry{}, s.logs...)}
}

// LiveTelemetry avoids copying the log buffer for menu-bar updates.
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

func (s *Service) startStreamsAt(authority string, secret string, done <-chan struct{}) {
	s.streamMu.Lock()
	s.streamGeneration++
	generation := s.streamGeneration
	s.telemetry = Telemetry{}
	s.logs = nil
	s.streamMu.Unlock()
	for _, kind := range []string{"traffic", "logs"} {
		go func(kind string) {
			for {
				select {
				case <-done:
					return
				default:
				}
				address := fmt.Sprintf("ws://%s/%s?token=%s", authority, kind, url.QueryEscape(secret))
				ws, err := websocket.Dial(address, "", "http://127.0.0.1/")
				if err == nil {
					for {
						if kind == "traffic" {
							var t Telemetry
							if websocket.JSON.Receive(ws, &t) != nil {
								break
							}
							s.streamMu.Lock()
							if s.streamGeneration == generation {
								t.Connected = true
								s.telemetry = t
							}
							s.streamMu.Unlock()
						} else {
							var l LogEntry
							if websocket.JSON.Receive(ws, &l) != nil {
								break
							}
							l.Time = time.Now().Format(time.RFC3339)
							s.streamMu.Lock()
							if s.streamGeneration == generation {
								s.logs = append(s.logs, l)
								if len(s.logs) > 500 {
									s.logs = append([]LogEntry(nil), s.logs[len(s.logs)-500:]...)
								}
							}
							s.streamMu.Unlock()
						}
					}
					ws.Close()
				}
				if kind == "traffic" {
					s.streamMu.Lock()
					if s.streamGeneration == generation {
						s.telemetry.Connected = false
					}
					s.streamMu.Unlock()
				}
				select {
				case <-done:
					return
				case <-time.After(time.Second):
				}
			}
		}(kind)
	}
}
