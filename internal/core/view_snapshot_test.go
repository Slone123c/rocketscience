package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCompactViewSnapshotsOmitBulkContent(t *testing.T) {
	p := NewProfiles(t.TempDir())
	if err := p.write(profileData{ActiveID: "active", MixedPort: 7891, SystemProxy: true, Profiles: []Profile{{ID: "active", Content: strings.Repeat("private-yaml", 10000), URL: "https://subscription.example/secret"}}}); err != nil {
		t.Fatal(err)
	}
	summary, err := p.Summary()
	if err != nil || summary.ActiveID != "active" || summary.MixedPort != 7891 || !summary.SystemProxy {
		t.Fatalf("summary: %+v %v", summary, err)
	}
	data, err := json.Marshal(summary)
	if err != nil || len(data) > 200 || strings.Contains(string(data), "secret") || strings.Contains(string(data), "private-yaml") {
		t.Fatal("summary copied bulk/private profile content")
	}
	s := New()
	s.telemetry = Telemetry{Up: 12, Down: 24, Connected: true}
	s.trafficSamples[0] = trafficSample{12, 24}
	s.trafficNext, s.trafficCount = 1, 1
	s.logs = []LogEntry{{Payload: strings.Repeat("large-log", 10000)}}
	monitor := s.Monitoring()
	data, err = json.Marshal(monitor)
	if err != nil || len(data) > 500 || strings.Contains(string(data), "large-log") {
		t.Fatal("monitoring copied logs")
	}
	monitor.History.Up[0] = -1
	if s.Monitoring().History.Up[0] != 12 || s.Streams().Logs[0].Payload != s.logs[0].Payload {
		t.Fatal("compact snapshot mutated shared state or removed logs")
	}
}
