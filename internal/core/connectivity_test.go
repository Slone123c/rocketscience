package core

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func startTestHTTPConnectProxy(t *testing.T, delay time.Duration, enabled *atomic.Bool) int {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if enabled != nil && !enabled.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		if delay > 0 {
			time.Sleep(delay)
		}
		if r.Method != http.MethodConnect {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		targetConn, err := net.Dial("tcp", r.Host)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			targetConn.Close()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		clientConn, _, err := hijacker.Hijack()
		if err != nil {
			targetConn.Close()
			return
		}
		_, _ = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		go func() {
			defer targetConn.Close()
			defer clientConn.Close()
			_, _ = io.Copy(targetConn, clientConn)
		}()
		go func() {
			defer targetConn.Close()
			defer clientConn.Close()
			_, _ = io.Copy(clientConn, targetConn)
		}()
	}))
	t.Cleanup(server.Close)
	_, rawPort, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		t.Fatal(err)
	}
	return port
}

func TestConnectivity(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", filepath.Join(t.TempDir(), "missing"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer server.Close()
	s := newTestService(t)
	defer s.Shutdown()
	result, err := s.CheckConnectivity(context.Background(), server.URL)
	if err != nil || result.Core {
		t.Fatal("stopped core should fail first step")
	}
	if err = s.Start(direct, freePort(t)); err != nil {
		t.Fatal(err)
	}
	result, err = s.CheckConnectivity(context.Background(), server.URL)
	if err != nil || !result.Core || !result.Request {
		t.Fatalf("check failed: %+v %v", result, err)
	}
	if _, err = s.CheckConnectivity(context.Background(), "file:///tmp/x"); err == nil {
		t.Fatal("bad URL accepted")
	}
}

func TestConnectivityDoesNotRecommendBuiltInDirectOrReject(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", filepath.Join(t.TempDir(), "missing"))
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	backupPort := startTestHTTPConnectProxy(t, 0, nil)
	s := newTestService(t)
	defer s.Shutdown()
	cfg := fmt.Sprintf("mode: rule\nlog-level: silent\nproxies:\n  - {name: dead, type: http, server: 127.0.0.1, port: 1}\n  - {name: backup, type: http, server: 127.0.0.1, port: %d}\nproxy-groups:\n  - {name: choose, type: select, proxies: [dead, DIRECT, REJECT, backup]}\nrules:\n  - MATCH,DIRECT\n", backupPort)
	if err := s.Start(cfg, freePort(t)); err != nil {
		t.Fatal(err)
	}
	result, err := s.CheckConnectivity(context.Background(), target.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Recommendations) != 1 || result.Recommendations[0].Node != "backup" {
		t.Fatalf("expected backup node recommendation (skipping DIRECT), got: %+v", result.Recommendations)
	}
}
