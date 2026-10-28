package core

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func metadataService(t *testing.T, handler http.HandlerFunc) *Service {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	_, raw, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatal(err)
	}
	return &Service{cmd: &exec.Cmd{}, port: port}
}

func TestProxyLatestMetadataAndUnknownNodes(t *testing.T) {
	s := metadataService(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"proxies":{"choose":{"type":"Selector","now":"fast","all":["fast","failed","untested","missing"]},"fast":{"type":"Shadowsocks","udp":true,"history":[{"time":"2026-01-01T00:00:00Z","delay":100},{"time":"2026-01-01T00:01:00Z","delay":25}]},"failed":{"type":"Trojan","udp":false,"history":[{"time":"2026-01-01T00:01:00Z","delay":0}]},"untested":{"type":"Direct","history":[]}}}`))
	})
	groups, err := s.Groups()
	if err != nil || len(groups) != 1 {
		t.Fatalf("groups: %+v %v", groups, err)
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
	for _, change := range []string{"stop", "restart", "configuration"} {
		t.Run(change, func(t *testing.T) {
			ready, release := make(chan struct{}), make(chan struct{})
			s := metadataService(t, func(w http.ResponseWriter, r *http.Request) {
				close(ready)
				<-release
				_, _ = w.Write([]byte(`{"node":12}`))
			})
			result := make(chan error, 1)
			go func() { _, err := s.TestGroup("group", "https://example.com", 1000); result <- err }()
			select {
			case <-ready:
			case <-time.After(2 * time.Second):
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
