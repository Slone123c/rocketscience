package core

import (
	"os"
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
