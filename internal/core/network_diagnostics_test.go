package core

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestNetworkDiagnosticsAndConfirmedRecommendation(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", filepath.Join(t.TempDir(), "missing"))
	probeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer probeServer.Close()

	var enabled atomic.Bool
	enabled.Store(true)
	goodPort := startTestHTTPConnectProxy(t, 0, &enabled)
	slowerPort := startTestHTTPConnectProxy(t, 40*time.Millisecond, &enabled)

	s := newTestService(t)
	defer s.Shutdown()
	cfg := fmt.Sprintf("mode: rule\nlog-level: silent\nproxies:\n  - {name: bad, type: http, server: 127.0.0.1, port: 1}\n  - {name: good, type: http, server: 127.0.0.1, port: %d}\n  - {name: slower, type: http, server: 127.0.0.1, port: %d}\nproxy-groups:\n  - {name: choose, type: select, proxies: [bad, good, slower]}\nrules:\n  - MATCH,DIRECT\n", goodPort, slowerPort)
	if err := s.Start(cfg, freePort(t)); err != nil {
		t.Fatal(err)
	}
	got, err := s.CheckConnectivity(probeServer.URL + "/probe")
	if err != nil || !got.Core || !got.Request || got.NodeStatus != "fail" || len(got.Recommendations) != 1 || got.Recommendations[0].Node != "good" {
		t.Fatalf("diagnosis: %+v %v", got, err)
	}
	proxies, err := inProcessProxiesMap()
	if err != nil || proxies["choose"].Now != "bad" {
		t.Fatal("diagnosis changed route without approval")
	}
	if err := s.SelectRecommendation("choose", "old", "good"); err == nil {
		t.Fatal("accepted stale recommendation")
	}
	if err := s.SelectRecommendation("choose", "bad", "missing"); err == nil {
		t.Fatal("accepted missing node")
	}
	if err := s.SelectRecommendation("choose", "bad", "good"); err != nil {
		t.Fatalf("approved selection failed: %v", err)
	}
	proxies, _ = inProcessProxiesMap()
	if proxies["choose"].Now != "good" {
		t.Fatalf("approved selection not applied: %q", proxies["choose"].Now)
	}
	got, err = s.CheckConnectivity(probeServer.URL + "/probe")
	if err != nil || got.NodeStatus != "ok" || len(got.Recommendations) != 0 {
		t.Fatalf("after selection: %+v %v", got, err)
	}
	enabled.Store(false)
	got, err = s.CheckConnectivity(probeServer.URL + "/probe")
	if err != nil || got.NodeStatus != "unknown" || !got.Request {
		t.Fatalf("failed test must be unknown: %+v %v", got, err)
	}
}
