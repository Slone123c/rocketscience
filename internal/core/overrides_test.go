package core

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestOverridesPersistAndApply(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO to test a real core")
	}
	s := newTestService(t)
	defer s.Shutdown()
	p := NewProfilesWithService(t.TempDir(), s)
	state, err := p.Save(Profile{Name: "test", Source: "file", Content: direct}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err = p.SetPort(freePort(t)); err != nil {
		t.Fatal(err)
	}
	if err = p.StartActive(); err != nil {
		t.Fatal(err)
	}
	o, err := p.Overrides()
	if err != nil {
		t.Fatal(err)
	}
	o.Groups[0].Rules = []OverrideRule{{"DOMAIN-SUFFIX", "example.com", "REJECT"}, {"DOMAIN", "missing.example", "not-present"}}
	o, err = p.SetOverrides(o.Groups, o.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if len(o.Skipped) != 1 || len(o.Origins) != 1 {
		t.Fatalf("incorrect attribution: %+v", o)
	}
	rules, err := s.Rules()
	if err != nil || len(rules) != 2 || rules[0].Proxy != "REJECT" || rules[1].Proxy != "DIRECT" {
		t.Fatalf("override order: %+v %v", rules, err)
	}
	stored, err := p.State()
	if err != nil || stored.Profiles[0].Content != direct || stored.ActiveID != state.ActiveID {
		t.Fatal("original profile modified")
	}
	if _, err = p.SetOverrides(o.Groups, o.Revision-1); err == nil {
		t.Fatal("stale revision accepted")
	}
	o.Groups[0].Rules[0].Payload = "bad,extra"
	if _, err = p.SetOverrides(o.Groups, o.Revision); err == nil {
		t.Fatal("injected rule accepted")
	}
	rules, err = s.Rules()
	if err != nil || rules[0].Proxy != "REJECT" {
		t.Fatal("bad update changed running rules")
	}
}
func TestMergePreservesAnchorsAndLeavesMissingPolicyOut(t *testing.T) {
	raw := "base: &base\n  - MATCH,DIRECT\nrules: *base\nproxy-groups:\n  - name: 代理\n    type: select\n    proxies: [DIRECT]\n"
	groups := []OverrideGroup{{ID: "x", Name: "x", Enabled: true, Rules: []OverrideRule{{"DOMAIN", "local.example", "代理"}, {"DOMAIN", "other.example", "missing"}}}}
	merged, policies, skipped, origins, err := mergeOverrides(raw, groups)
	if err != nil || len(policies) != 1 || len(skipped) != 1 || len(origins) != 1 {
		t.Fatalf("merge: %v %v %v %v", policies, skipped, origins, err)
	}
	if !strings.Contains(merged, "DOMAIN,local.example,代理") || strings.Contains(merged, "DOMAIN,other.example,missing") || !strings.Contains(merged, "&base") {
		t.Fatal("merge did not preserve content")
	}
}

func TestOverridesApplyToUnsavedDraft(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO to test a real core")
	}
	s := newTestService(t)
	defer s.Shutdown()
	p := NewProfilesWithService(t.TempDir(), s)
	state, err := p.Overrides()
	if err != nil {
		t.Fatal(err)
	}
	state.Groups[0].Rules = []OverrideRule{{"DOMAIN", "draft.example", "REJECT"}}
	if _, err = p.SetOverrides(state.Groups, state.Revision); err != nil {
		t.Fatal(err)
	}
	if err = p.StartDraft(direct, freePort(t)); err != nil {
		t.Fatal(err)
	}
	rules, err := s.Rules()
	if err != nil || len(rules) != 2 || rules[0].Proxy != "REJECT" {
		t.Fatalf("draft rules: %+v %v", rules, err)
	}
}
func TestSubscriptionReloadRetainsOverrides(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO to test a real core")
	}
	response := strings.Replace(direct, "MATCH,DIRECT", "MATCH,REJECT", 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(response)) }))
	defer server.Close()
	s := newTestService(t)
	defer s.Shutdown()
	p := NewProfilesWithService(t.TempDir(), s)
	state, err := p.Save(Profile{Name: "test", Source: "url", URL: server.URL, Content: direct}, "")
	if err != nil {
		t.Fatal(err)
	}
	o, err := p.Overrides()
	if err != nil {
		t.Fatal(err)
	}
	o.Groups[0].Rules = []OverrideRule{{"DOMAIN", "override.example", "DIRECT"}}
	if _, err = p.SetOverrides(o.Groups, o.Revision); err != nil {
		t.Fatal(err)
	}
	if err = p.SetPort(freePort(t)); err != nil {
		t.Fatal(err)
	}
	if err = p.StartActive(); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Update(state.ActiveID); err != nil {
		t.Fatal(err)
	}
	rules, err := s.Rules()
	if err != nil || len(rules) != 2 || rules[0].Proxy != "DIRECT" || rules[1].Proxy != "REJECT" {
		t.Fatalf("rules after subscription update: %+v %v", rules, err)
	}
	stored, err := p.State()
	if err != nil || stored.Profiles[0].Content != response {
		t.Fatal("original subscription should not include override")
	}
}
