package core

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConnectivityRejectsChangesWhileRequestIsPending(t *testing.T) {
	for _, change := range []string{"stop", "restart", "configuration", "settings", "mode", "selection"} {
		t.Run(change, func(t *testing.T) {
			probe, release := make(chan struct{}), make(chan struct{})
			var mu sync.Mutex
			node := "first"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/version":
					_, _ = w.Write([]byte(`{"version":"test"}`))
				case "/proxies":
					mu.Lock()
					selected := node
					mu.Unlock()
					_ = json.NewEncoder(w).Encode(map[string]any{"proxies": map[string]any{"choose": Proxy{Name: "choose", Type: "Selector", Now: selected, All: []string{"first", "second"}}}})
				case "/group/choose/delay":
					_, _ = w.Write([]byte(`{"first":10,"second":20}`))
				case "/probe":
					close(probe)
					<-release
					w.WriteHeader(http.StatusNoContent)
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			_, rawPort, _ := net.SplitHostPort(server.Listener.Addr().String())
			port, _ := strconv.Atoi(rawPort)
			s := &Service{cmd: &exec.Cmd{}, port: port, mixedPort: port, secret: "test", startedAt: time.Now()}
			done := make(chan error, 1)
			go func() {
				result, err := s.CheckConnectivity(server.URL + "/probe")
				if err == nil || result.Core || result.Request || len(result.Recommendations) != 0 {
					done <- nil
					return
				}
				done <- err
			}()
			select {
			case <-probe:
			case <-time.After(5 * time.Second):
				close(release)
				t.Fatal("diagnostics did not reach the test request")
			}
			s.mu.Lock()
			switch change {
			case "stop":
				s.cmd = nil
			case "restart":
				s.startedAt = s.startedAt.Add(time.Second)
			case "configuration":
				s.currentContent = "changed"
			case "settings":
				s.coreOptions.LogLevel = ptr("debug")
			case "mode":
				s.modeRevision++
			case "selection":
				mu.Lock()
				node = "second"
				mu.Unlock()
			}
			s.mu.Unlock()
			close(release)
			select {
			case err := <-done:
				if err == nil || !strings.Contains(err.Error(), "请重新检查") {
					t.Fatalf("stale diagnostics accepted: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("diagnostics did not finish")
			}
		})
	}
}
