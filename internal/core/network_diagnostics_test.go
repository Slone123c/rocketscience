package core

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strconv"
	"sync"
	"testing"
)

func TestNetworkDiagnosticsAndConfirmedRecommendation(t *testing.T) {
	var mu sync.Mutex
	current, putCount, groupStatus := "bad", 0, http.StatusOK
	delays := map[string]int{"good": 123, "slower": 200}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/version":
			_ = json.NewEncoder(w).Encode(map[string]string{"version": "test"})
		case "/proxies":
			_ = json.NewEncoder(w).Encode(map[string]any{"proxies": map[string]any{"choose": Proxy{Name: "choose", Type: "Selector", Now: current, All: []string{"bad", "good", "slower"}}}})
		case "/group/choose/delay":
			if groupStatus != 200 {
				w.WriteHeader(groupStatus)
				return
			}
			_ = json.NewEncoder(w).Encode(delays)
		case "/proxies/choose":
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode(Proxy{Name: "choose", Type: "Selector", Now: current, All: []string{"bad", "good", "slower"}})
				return
			}
			var body struct {
				Name string `json:"name"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			current = body.Name
			putCount++
			w.WriteHeader(204)
		case "/probe":
			w.WriteHeader(204)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	_, rawPort, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{cmd: &exec.Cmd{}, port: port, mixedPort: port, secret: "test"}
	got, err := s.CheckConnectivity(server.URL + "/probe")
	if err != nil || !got.Core || !got.Request || got.NodeStatus != "fail" || len(got.Recommendations) != 1 || got.Recommendations[0].Node != "good" {
		t.Fatalf("diagnosis: %+v %v", got, err)
	}
	if putCount != 0 || current != "bad" {
		t.Fatal("diagnosis changed route without approval")
	}
	if err := s.SelectRecommendation("choose", "old", "good"); err == nil || putCount != 0 {
		t.Fatal("accepted stale recommendation")
	}
	if err := s.SelectRecommendation("choose", "bad", "missing"); err == nil || putCount != 0 {
		t.Fatal("accepted missing node")
	}
	if err := s.SelectRecommendation("choose", "bad", "good"); err != nil || current != "good" || putCount != 1 {
		t.Fatalf("approved selection: %q %d %v", current, putCount, err)
	}
	got, err = s.CheckConnectivity(server.URL + "/probe")
	if err != nil || got.NodeStatus != "ok" || len(got.Recommendations) != 0 {
		t.Fatalf("after selection: %+v %v", got, err)
	}
	mu.Lock()
	groupStatus = 500
	mu.Unlock()
	got, err = s.CheckConnectivity(server.URL + "/probe")
	if err != nil || got.NodeStatus != "unknown" || !got.Request {
		t.Fatalf("failed test must be unknown: %+v %v", got, err)
	}
}
