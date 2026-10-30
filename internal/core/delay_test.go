package core

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestGroupDelay(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", filepath.Join(t.TempDir(), "missing"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer server.Close()
	s := newTestService(t)
	defer s.Shutdown()
	cfg := "mode: rule\nlog-level: silent\nproxy-groups:\n  - name: 选择\n    type: select\n    proxies: [DIRECT, REJECT]\nrules:\n  - MATCH,选择\n"
	if err := s.Start(cfg, freePort(t)); err != nil {
		t.Fatal(err)
	}
	result, err := s.TestGroup("选择", server.URL, 3000)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result["DIRECT"]; !ok {
		t.Fatalf("direct result missing: %v", result)
	}
	groups, err := s.Groups()
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range groups {
		if g.Name == "选择" && g.Now != "DIRECT" {
			t.Fatal("speed test changed route")
		}
		if g.Name == "选择" {
			found := false
			for _, node := range g.Nodes {
				if node.Name == "DIRECT" {
					found = true
					if node.Delay == nil || node.DelayAt == "" || node.Type != "Direct" {
						t.Fatalf("real history not mapped: %+v", node)
					}
				}
			}
			if !found {
				t.Fatal("node metadata missing after real speed test")
			}
		}
	}
	if _, err = s.TestGroup("选择", "file:///tmp/test", 1000); err == nil {
		t.Fatal("invalid test URL accepted")
	}
}
