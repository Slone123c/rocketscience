package core

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestProxySelectionAndMode(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", filepath.Join(t.TempDir(), "missing"))
	s := newTestService(t)
	defer s.Shutdown()
	config := "mode: rule\nlog-level: silent\nproxy-groups:\n  - name: 选择\n    type: select\n    proxies: [DIRECT, REJECT]\n  - name: 广告\n    type: select\n    proxies: [REJECT, DIRECT]\nrules:\n  - DOMAIN,ads.example,广告\n  - DOMAIN,a.example,选择\n  - MATCH,选择\n"
	if err := s.Start(config, freePort(t)); err != nil {
		t.Fatal(err)
	}
	groups, err := s.Groups()
	if err != nil {
		t.Fatal(err)
	}
	// Configuration order, not name order: 选择 sorts after 广告 by code point.
	if len(groups) < 3 || groups[0].Name != "选择" || groups[0].Type != "Selector" || groups[1].Name != "广告" || groups[len(groups)-1].Name != "GLOBAL" {
		t.Fatalf("group order: %+v", groups)
	}
	if groups[0].Rules != 2 || !groups[0].Fallback || groups[1].Rules != 1 || groups[1].Fallback {
		t.Fatalf("rule targets: %+v", groups)
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
	t.Setenv("ROCKETSCIENCE_MIHOMO", filepath.Join(t.TempDir(), "missing"))
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

func TestRuleTargetsFromConfig(t *testing.T) {
	rules := configRules([]string{"DOMAIN-SUFFIX,example.com,媒体", "IP-CIDR,10.0.0.0/8,DIRECT,no-resolve", "AND,((DOMAIN,a.com),(NETWORK,UDP)),媒体", "GEOIP,CN,DIRECT", "MATCH,兜底", "bad"})
	groups := []Proxy{{Name: "媒体"}, {Name: "兜底"}, {Name: "空闲"}}
	annotateRuleTargets(groups, rules)
	if groups[0].Rules != 2 || groups[0].Fallback || groups[1].Rules != 1 || !groups[1].Fallback || groups[2].Rules != 0 {
		t.Fatalf("targets: %+v", groups)
	}
}

func TestModeChosenWhileStoppedAppliesOnStartAndSurvivesStop(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", filepath.Join(t.TempDir(), "missing"))
	s := newTestService(t)
	defer s.Shutdown()
	if err := s.SetMode("bogus"); err == nil {
		t.Fatal("invalid mode accepted while stopped")
	}
	if err := s.SetMode("global"); err != nil {
		t.Fatalf("set while stopped: %v", err)
	}
	if st := s.Status(); st.Running || st.Mode != "global" {
		t.Fatalf("stopped choice: %+v", st)
	}
	config := "mode: rule\nlog-level: silent\n"
	if err := s.Start(config, freePort(t)); err != nil {
		t.Fatal(err)
	}
	if st := s.Status(); !st.Running || st.Mode != "global" {
		t.Fatalf("after start: %+v", st)
	}
	if err := s.SetMode("direct"); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if st := s.Status(); st.Running || st.Mode != "direct" {
		t.Fatalf("after stop: %+v", st)
	}
	if err := s.Start(config, freePort(t)); err != nil {
		t.Fatal(err)
	}
	if st := s.Status(); !st.Running || st.Mode != "direct" {
		t.Fatalf("after second start: %+v", st)
	}
}
