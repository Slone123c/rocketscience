package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProxyLatestMetadataAndUnknownNodes(t *testing.T) {
	var parsed struct {
		Proxies map[string]proxyDetail `json:"proxies"`
	}
	raw := `{"proxies":{"choose":{"type":"Selector","now":"fast","all":["fast","failed","untested","missing"]},"fast":{"type":"Shadowsocks","udp":true,"history":[{"time":"2026-01-01T00:00:00Z","delay":100},{"time":"2026-01-01T00:01:00Z","delay":25}]},"failed":{"type":"Trojan","udp":false,"history":[{"time":"2026-01-01T00:01:00Z","delay":0}]},"untested":{"type":"Direct","history":[]}}}`
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatal(err)
	}
	groups := buildProxyGroups(parsed.Proxies, nil)
	if len(groups) != 1 {
		t.Fatalf("groups: %+v", groups)
	}
	g := groups[0]
	if g.Name != "choose" || len(g.Nodes) != 4 || g.Now != "fast" {
		t.Fatalf("bad group metadata: %+v", g)
	}
	if n := g.Nodes[0]; n.Type != "Shadowsocks" || n.UDP == nil || !*n.UDP || n.Delay == nil || *n.Delay != 25 || n.DelayAt != "2026-01-01T00:01:00Z" {
		t.Fatalf("bad latest history: %+v", n)
	}
	if g.Nodes[1].Delay == nil || *g.Nodes[1].Delay != 0 || g.Nodes[2].Delay != nil || g.Nodes[3].Delay != nil {
		t.Fatal("timeout and untested nodes conflated")
	}
	data, _ := json.Marshal(groups)
	if strings.Contains(string(data), "history") || strings.Contains(string(data), "00:00:00") {
		t.Fatal("full history leaked into page snapshot")
	}
}

func TestGroupDelayRejectsChangedSessionOrConfiguration(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", filepath.Join(t.TempDir(), "missing"))
	for _, change := range []string{"stop", "restart", "configuration"} {
		t.Run(change, func(t *testing.T) {
			ready, release := make(chan struct{}), make(chan struct{})
			targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(ready)
				<-release
				w.WriteHeader(http.StatusNoContent)
			}))
			defer targetServer.Close()
			s := newTestService(t)
			defer s.Shutdown()
			cfg := "mode: rule\nlog-level: silent\nproxy-groups:\n  - name: group\n    type: select\n    proxies: [DIRECT]\nrules:\n  - MATCH,group\n"
			if err := s.Start(cfg, freePort(t)); err != nil {
				t.Fatal(err)
			}
			result := make(chan error, 1)
			go func() { _, err := s.TestGroup("group", targetServer.URL, 2000); result <- err }()
			select {
			case <-ready:
			case <-time.After(2 * time.Second):
				close(release)
				t.Fatal("delay request did not start")
			}
			s.mu.Lock()
			switch change {
			case "stop":
				s.cmd = nil
			case "restart":
				s.startedAt = time.Now()
			case "configuration":
				s.currentContent = "changed"
			}
			s.mu.Unlock()
			close(release)
			select {
			case err := <-result:
				if err == nil || !strings.Contains(err.Error(), "已改变") {
					t.Fatalf("stale result accepted: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("delay did not finish")
			}
		})
	}
}
