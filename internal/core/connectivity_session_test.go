package core

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConnectivityRejectsChangesWhileRequestIsPending(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", filepath.Join(t.TempDir(), "missing"))
	for _, change := range []string{"stop", "restart", "configuration", "settings", "mode", "selection"} {
		t.Run(change, func(t *testing.T) {
			probe, release := make(chan struct{}), make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodHead {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				close(probe)
				<-release
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			s := newTestService(t)
			defer s.Shutdown()
			cfg := "mode: rule\nlog-level: silent\nproxy-groups:\n  - name: choose\n    type: select\n    proxies: [DIRECT, REJECT]\nrules:\n  - MATCH,choose\n"
			if err := s.Start(cfg, freePort(t)); err != nil {
				t.Fatal(err)
			}
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
				_ = selectInProcessProxy("choose", "REJECT")
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
