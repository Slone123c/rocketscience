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

func TestProxySelectionAndMode(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO to test a real core")
	}
	s := newTestService(t)
	defer s.Shutdown()
	config := "mode: rule\nlog-level: silent\nproxy-groups:\n  - name: 选择\n    type: select\n    proxies: [DIRECT, REJECT]\nrules:\n  - MATCH,选择\n"
	if err := s.Start(config, freePort(t)); err != nil {
		t.Fatal(err)
	}
	groups, err := s.Groups()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, g := range groups {
		if g.Name == "选择" && g.Type == "Selector" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing group: %+v", groups)
	}
	if err = s.Select("选择", "REJECT"); err != nil {
		t.Fatal(err)
	}
	groups, err = s.Groups()
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range groups {
		if g.Name == "选择" && g.Now != "REJECT" {
			t.Fatalf("selection did not persist: %+v", g)
		}
	}
	if err = s.SetMode("direct"); err != nil {
		t.Fatal(err)
	}
	if mode := s.Status().Mode; mode != "direct" {
		t.Fatalf("mode: %s", mode)
	}
	if err = s.SetMode("bad"); err == nil {
		t.Fatal("bad mode accepted")
	}
}

func TestSelectAndSetModeCloseStaleConnections(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO to test a real core")
	}
	s := newTestService(t)
	defer s.Shutdown()
	port := freePort(t)
	config := "mode: rule\nlog-level: silent\nproxy-groups:\n  - name: 选择\n    type: select\n    proxies: [DIRECT, REJECT]\nrules:\n  - MATCH,选择\n"
	if err := s.Start(config, port); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	defer close(release)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	proxyURL, _ := url.Parse("http://127.0.0.1:" + strconv.Itoa(port))
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := http.Client{Transport: transport, Timeout: 5 * time.Second}

	openConn := func() chan struct{} {
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
			t.Fatal("request did not reach server")
		}
		return done
	}

	done1 := openConn()
	snap, err := s.Connections()
	if err != nil || len(snap.Connections) == 0 {
		t.Fatalf("expected active connection before Select, got: %+v %v", snap, err)
	}
	if err := s.Select("选择", "REJECT"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done1:
	case <-time.After(3 * time.Second):
		t.Fatal("Select did not close active connection routed through group")
	}

	if err := s.Select("选择", "DIRECT"); err != nil {
		t.Fatal(err)
	}
	done2 := openConn()
	if err := s.SetMode("direct"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done2:
	case <-time.After(3 * time.Second):
		t.Fatal("SetMode did not close active connection")
	}
}
