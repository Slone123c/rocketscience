package core

import (
	"os"
	"strings"
	"testing"
)

// PRX-03: manually chosen Selector nodes must survive stop -> start.
func TestSelectionsSurviveStopStart(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO")
	}
	s := newTestService(t)
	defer s.Shutdown()
	content := "mode: rule\nlog-level: silent\nproxy-groups:\n  - name: pick\n    type: select\n    proxies: [DIRECT, REJECT]\nrules: ['MATCH,DIRECT']\n"
	port := freePort(t)
	if err := s.Start(content, port); err != nil {
		t.Fatal(err)
	}
	if err := s.Select("pick", "REJECT"); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(content, port); err != nil {
		t.Fatal(err)
	}
	groups, err := s.Groups()
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range groups {
		if g.Name == "pick" && g.Now != "REJECT" {
			t.Fatalf("selection reset to %q", g.Now)
		}
	}
}

// CFG-01: Restart re-serializes the stored profile; numeric-looking text must stay intact.
func TestRestartKeepsNumericLookingScalars(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO")
	}
	s := newTestService(t)
	defer s.Shutdown()
	content := "mode: rule\nlog-level: silent\nx-id: 01234567\nproxy-groups:\n  - name: pick\n    type: select\n    proxies: [DIRECT]\nrules: ['MATCH,DIRECT']\n"
	if err := s.Start(content, freePort(t)); err != nil {
		t.Fatal(err)
	}
	if err := s.Restart(); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	got := s.currentContent
	s.mu.Unlock()
	if !strings.Contains(got, "x-id: 01234567") {
		t.Fatalf("restart rewrote profile:\n%s", got)
	}
}
