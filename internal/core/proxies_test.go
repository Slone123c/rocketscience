package core

import (
	"os"
	"testing"
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
