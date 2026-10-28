package core

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestProfileCommitLiveSaveAndSwitch(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO")
	}
	s := newTestService(t)
	defer s.Shutdown()
	p := NewProfilesWithService(t.TempDir(), s)
	first, err := p.Save(Profile{Name: "first", Source: "file", Content: direct}, "")
	if err != nil {
		t.Fatal(err)
	}
	firstID := first.ActiveID
	second, err := p.Save(Profile{Name: "second", Source: "file", Content: strings.Replace(direct, "MATCH,DIRECT", "MATCH,REJECT", 1)}, "")
	if err != nil {
		t.Fatal(err)
	}
	secondID := second.Profiles[1].ID
	if err := p.SetPort(freePort(t)); err != nil {
		t.Fatal(err)
	}
	if err := p.StartActive(); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMode("direct"); err != nil {
		t.Fatal(err)
	}
	before := s.Status()
	selected, err := p.Use(secondID)
	if err != nil || selected.ActiveID != secondID {
		t.Fatalf("live switch: %+v %v", selected, err)
	}
	rules, err := s.Rules()
	if err != nil || len(rules) != 1 || rules[0].Proxy != "REJECT" {
		t.Fatalf("switch rules: %+v %v", rules, err)
	}
	after := s.Status()
	if !after.Running || after.StartedAt != before.StartedAt || after.MixedPort != before.MixedPort || after.Controller != before.Controller || after.Mode != "direct" {
		t.Fatalf("switch replaced process/settings: %+v", after)
	}
	v := selected.Profiles[1]
	oldVersion := v.UpdatedAt.Format(time.RFC3339Nano)
	v.Content = direct
	saved, err := p.Save(v, oldVersion)
	if err != nil {
		t.Fatal(err)
	}
	if !saved.Profiles[1].UpdatedAt.After(v.UpdatedAt) {
		t.Fatal("save did not advance editing version")
	}
	if _, err := p.Save(v, oldVersion); err == nil {
		t.Fatal("stale editor overwrote newer saved version")
	}
	rules, err = s.Rules()
	if err != nil || rules[0].Proxy != "DIRECT" {
		t.Fatalf("save not applied: %+v %v", rules, err)
	}
	v = saved.Profiles[0]
	v.Content = strings.Replace(direct, "MATCH,DIRECT", "MATCH,REJECT", 1)
	if _, err := p.Save(v, v.UpdatedAt.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	rules, _ = s.Rules()
	if rules[0].Proxy != "DIRECT" {
		t.Fatal("inactive save changed running configuration")
	}
	if _, err := p.Use(firstID); err != nil {
		t.Fatal(err)
	}
	rules, _ = s.Rules()
	if rules[0].Proxy != "REJECT" {
		t.Fatal("switch ignored inactive edit")
	}
	if _, err := p.Use("missing"); err == nil {
		t.Fatal("missing profile activated")
	}
	if _, err := p.Delete(secondID); err != nil {
		t.Fatalf("cannot delete inactive profile while running: %v", err)
	}
}

func TestProfileCommitRollbackAndNodeChoices(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO")
	}
	s := newTestService(t)
	defer s.Shutdown()
	content := "mode: rule\nlog-level: silent\nproxy-groups:\n  - name: select\n    type: select\n    proxies: [DIRECT, REJECT]\nrules: ['MATCH,DIRECT']\n"
	if err := s.Start(content, freePort(t)); err != nil {
		t.Fatal(err)
	}
	if err := s.Select("select", "REJECT"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMode("global"); err != nil {
		t.Fatal(err)
	}
	next := strings.Replace(content, "MATCH,DIRECT", "MATCH,REJECT", 1)
	if err := s.commitConfiguration(next, func() error { return errors.New("disk full") }); err == nil {
		t.Fatal("persistence failure ignored")
	}
	rules, _ := s.Rules()
	if rules[0].Proxy != "DIRECT" || s.Status().Mode != "global" {
		t.Fatal("persistence failure did not roll back")
	}
	groups, err := s.Groups()
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range groups {
		if g.Name == "select" && g.Now != "REJECT" {
			t.Fatal("rollback lost manual node")
		}
	}
	s.mu.Lock()
	if s.currentContent != content {
		t.Error("rollback changed restart content")
	}
	s.mu.Unlock()
	if err := s.commitConfiguration(next, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	groups, _ = s.Groups()
	for _, g := range groups {
		if g.Name == "select" && g.Now != "REJECT" {
			t.Fatal("successful reload lost manual node")
		}
	}
	called := false
	if err := s.commitConfiguration("rules: [", func() error { called = true; return nil }); err == nil || called {
		t.Fatal("invalid reload persisted")
	}
	rules, _ = s.Rules()
	if rules[0].Proxy != "REJECT" {
		t.Fatal("failed reload damaged previous config")
	}
	if err := s.commitConfiguration(direct, func() error { return nil }); err != nil {
		t.Fatalf("removed group should not prevent switching: %v", err)
	}
}

func TestProfileCommitOverridesAndInvalidSave(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO")
	}
	s := newTestService(t)
	defer s.Shutdown()
	p := NewProfilesWithService(t.TempDir(), s)
	state, err := p.Save(Profile{Name: "test", Source: "file", Content: direct}, "")
	if err != nil {
		t.Fatal(err)
	}
	o, err := p.Overrides()
	if err != nil {
		t.Fatal(err)
	}
	o.Groups[0].Rules = []OverrideRule{{Type: "DOMAIN", Payload: "example.com", Policy: "DIRECT"}}
	if _, err := p.SetOverrides(o.Groups, o.Revision); err != nil {
		t.Fatal(err)
	}
	if err := p.SetPort(freePort(t)); err != nil {
		t.Fatal(err)
	}
	if err := p.StartActive(); err != nil {
		t.Fatal(err)
	}
	v := state.Profiles[0]
	v.Content = strings.Replace(direct, "MATCH,DIRECT", "MATCH,REJECT", 1)
	state, err = p.Save(v, v.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	rules, _ := s.Rules()
	if len(rules) != 2 || rules[0].Payload != "example.com" || rules[1].Proxy != "REJECT" {
		t.Fatalf("override lost on save: %+v", rules)
	}
	v = state.Profiles[0]
	v.Content = "rules: ["
	if _, err := p.Save(v, v.UpdatedAt.Format(time.RFC3339Nano)); err == nil {
		t.Fatal("invalid active save accepted")
	}
	again, _ := p.State()
	if again.Profiles[0].Content != state.Profiles[0].Content {
		t.Fatal("invalid save changed disk")
	}
	rules, _ = s.Rules()
	if len(rules) != 2 || rules[1].Proxy != "REJECT" {
		t.Fatal("invalid save changed core")
	}
}
