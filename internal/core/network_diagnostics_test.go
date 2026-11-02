package core

import (
	"context"
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
	var localOnline atomic.Bool
	localOnline.Store(true)
	probeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !localOnline.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer probeServer.Close()

	var enabled atomic.Bool
	enabled.Store(true)
	goodPort := startTestHTTPConnectProxy(t, 0, &enabled)
	slowerPort := startTestHTTPConnectProxy(t, 40*time.Millisecond, &enabled)

	s := newTestService(t)
	defer s.Shutdown()
	// Include a secondary group with only a dead node to verify that one working node across all nodes suffices.
	cfg := fmt.Sprintf("mode: rule\nlog-level: silent\nproxies:\n  - {name: bad, type: http, server: 127.0.0.1, port: 1}\n  - {name: good, type: http, server: 127.0.0.1, port: %d}\n  - {name: slower, type: http, server: 127.0.0.1, port: %d}\nproxy-groups:\n  - {name: choose, type: select, proxies: [bad, good, slower]}\n  - {name: secondary, type: select, proxies: [bad]}\nrules:\n  - MATCH,choose\n", goodPort, slowerPort)
	if err := s.Start(cfg, freePort(t)); err != nil {
		t.Fatal(err)
	}

	// Step 1 passes (local network ok), Step 3 finds "good" and auto-switches "choose" from "bad" to "good", Step 2 passes.
	got, err := s.CheckConnectivity(context.Background(), probeServer.URL+"/probe")
	if err != nil || !got.Local || got.LocalStatus != "ok" || !got.Core || got.TakeoverStatus != "ok" || !got.Request || got.NodeStatus != "ok" || got.VerifiedNode != "good" || !got.Switched {
		t.Fatalf("diagnosis: %+v %v", got, err)
	}
	proxies, err := inProcessProxiesMap()
	if err != nil || proxies["choose"].Now != "good" {
		t.Fatalf("expected choose to auto-switch to good, got %q (err=%v)", proxies["choose"].Now, err)
	}

	// Validate SelectRecommendation guards as well.
	if err := s.SelectRecommendation("choose", "old", "slower"); err == nil {
		t.Fatal("accepted stale recommendation")
	}
	if err := s.SelectRecommendation("choose", "good", "missing"); err == nil {
		t.Fatal("accepted missing node")
	}
	if err := s.SelectRecommendation("choose", "good", "slower"); err != nil {
		t.Fatalf("approved selection failed: %v", err)
	}
	if err := s.SelectRecommendation("choose", "slower", "good"); err != nil {
		t.Fatalf("restore selection failed: %v", err)
	}

	// Second check when already on "good" succeeds without needing another switch.
	got, err = s.CheckConnectivity(context.Background(), probeServer.URL+"/probe")
	if err != nil || got.NodeStatus != "ok" || got.VerifiedNode != "good" || got.Switched {
		t.Fatalf("after selection: %+v %v", got, err)
	}

	// Step 2: When all proxy nodes fail and traffic through proxy is broken, while local network is still online,
	// CheckConnectivity must detect that proxy caused disconnect and automatically exit the proxy.
	enabled.Store(false)
	got, err = s.CheckConnectivity(context.Background(), probeServer.URL+"/probe")
	if err != nil || !got.Local || got.TakeoverStatus != "fail" || !got.AutoExited || got.Core || got.Request || got.NodeStatus != "fail" {
		t.Fatalf("expected auto-exit when proxy causes disconnect: %+v %v", got, err)
	}
	if s.Status().Running {
		t.Fatal("proxy should have exited automatically when it caused disconnect")
	}

	// Step 1: When local network itself is down in no-proxy state, Check 1 fails and proxy remains off.
	localOnline.Store(false)
	got, err = s.CheckConnectivity(context.Background(), probeServer.URL+"/probe")
	if err != nil || got.Local || got.LocalStatus != "fail" || got.TakeoverStatus != "skip" || got.NodeStatus != "skip" || got.Core {
		t.Fatalf("expected local network failure in step 1: %+v %v", got, err)
	}
	if s.Status().Running {
		t.Fatal("proxy must remain off when local network is offline")
	}
}
