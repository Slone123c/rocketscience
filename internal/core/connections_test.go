package core

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestConnectionsAndRules(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO to test a real core")
	}
	s := newTestService(t)
	defer s.Shutdown()
	port := freePort(t)
	if err := s.Start(direct, port); err != nil {
		t.Fatal(err)
	}
	rules, err := s.Rules()
	if err != nil || len(rules) != 1 || rules[0].Proxy != "DIRECT" {
		t.Fatalf("rules: %+v, %v", rules, err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-release }))
	defer server.Close()
	proxy, _ := url.Parse("http://127.0.0.1:" + strconv.Itoa(port))
	client := http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxy)}, Timeout: 4 * time.Second}
	done := make(chan struct{})
	go func() {
		res, _ := client.Get(server.URL)
		if res != nil {
			res.Body.Close()
		}
		close(done)
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("request did not reach server")
	}
	snap, err := s.Connections()
	if err != nil || len(snap.Connections) == 0 {
		close(release)
		t.Fatalf("connections: %+v, %v", snap, err)
	}
	if err = s.CloseConnection(snap.Connections[0].ID); err != nil {
		close(release)
		t.Fatal(err)
	}
	close(release)
	<-done
}

func TestCloseAllConnections(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO")
	}
	s := newTestService(t)
	defer s.Shutdown()
	if err := s.CloseAllConnections(); err == nil {
		t.Fatal("closed while stopped")
	}
	port := freePort(t)
	if err := s.Start(direct, port); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer close(release)
	proxy, _ := url.Parse("http://127.0.0.1:" + strconv.Itoa(port))
	transport := &http.Transport{Proxy: http.ProxyURL(proxy), DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := http.Client{Transport: transport, Timeout: 8 * time.Second}
	done := make(chan struct{}, 2)
	for i := 0; i < 2; i++ {
		go func() {
			res, _ := client.Get(server.URL)
			if res != nil {
				res.Body.Close()
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(4 * time.Second):
			t.Fatal("requests did not start")
		}
	}
	snapshot, err := s.Connections()
	if err != nil || len(snapshot.Connections) < 2 {
		t.Fatalf("before close: %+v %v", snapshot, err)
	}
	if err = s.CloseAllConnections(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(4 * time.Second):
			t.Fatal("connection stayed open")
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		snapshot, err = s.Connections()
		if err == nil && len(snapshot.Connections) == 0 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("connections remained: %+v %v", snapshot, err)
}
