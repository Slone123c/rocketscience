package core

import (
	"os"
	"path/filepath"
	"testing"
)

const selectionConfig = `proxies:
  - {name: 东京, type: ss, server: 127.0.0.1, port: 1, cipher: aes-128-gcm, password: x}
  - {name: 大阪, type: ss, server: 127.0.0.1, port: 2, cipher: aes-128-gcm, password: x}
proxy-groups:
  - {name: 自动, type: url-test, proxies: [东京, 大阪], url: 'http://127.0.0.1:1', interval: 3600}
  - {name: 手选, type: select, proxies: [东京, 大阪, DIRECT]}
rules:
  - MATCH,手选
`

func previewNowOf(t *testing.T, p *Profiles, group string) string {
	t.Helper()
	groups, err := p.PreviewGroups()
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range groups {
		if g.Name == group {
			return g.Now
		}
	}
	t.Fatalf("missing group %s", group)
	return ""
}

func TestSelectPreviewWhileStopped(t *testing.T) {
	dir := t.TempDir()
	p := NewProfiles(dir)
	if _, err := p.Save(Profile{Name: "选择", Source: "file", Content: selectionConfig}, ""); err != nil {
		t.Fatal(err)
	}
	if now := previewNowOf(t, p, "手选"); now != "东京" {
		t.Fatalf("default is the first member, got %q", now)
	}
	if err := p.SelectPreview("手选", "大阪"); err != nil {
		t.Fatal(err)
	}
	if now := previewNowOf(t, p, "手选"); now != "大阪" {
		t.Fatalf("saved choice not shown, got %q", now)
	}
	// A fresh Profiles reads the same file, as after an app restart.
	if now := previewNowOf(t, NewProfiles(dir), "手选"); now != "大阪" {
		t.Fatalf("choice lost across restart, got %q", now)
	}
	for _, bad := range [][2]string{{"手选", "纽约"}, {"不存在", "东京"}, {"自动", "东京"}, {unassignedGroupName, "东京"}} {
		if err := p.SelectPreview(bad[0], bad[1]); err == nil {
			t.Fatalf("accepted %v", bad)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "selections.json")); err != nil {
		t.Fatal(err)
	}
}

func TestSelectPreviewAppliesOnStartAndFollowsTheCore(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO to test a real core")
	}
	dir := t.TempDir()
	s := newTestService(t)
	defer s.Shutdown()
	p := NewProfilesWithService(dir, s)
	if err := p.SetPort(freePort(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Save(Profile{Name: "选择", Source: "file", Content: selectionConfig}, ""); err != nil {
		t.Fatal(err)
	}
	if err := p.SelectPreview("手选", "大阪"); err != nil {
		t.Fatal(err)
	}
	if err := p.StartActive(); err != nil {
		t.Fatal(err)
	}
	if err := p.SelectPreview("手选", "东京"); err == nil {
		t.Fatal("preview selection accepted while running")
	}
	running := func() string {
		groups, err := s.Groups()
		if err != nil {
			t.Fatal(err)
		}
		for _, g := range groups {
			if g.Name == "手选" {
				return g.Now
			}
		}
		return ""
	}
	if now := running(); now != "大阪" {
		t.Fatalf("start ignored the stopped choice: %q", now)
	}
	if err := s.Select("手选", "DIRECT"); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if now := previewNowOf(t, NewProfiles(dir), "手选"); now != "DIRECT" {
		t.Fatalf("running choice not saved: %q", now)
	}
	// A saved node that no longer exists: the core falls back to its default,
	// and the saved choice follows what the core actually uses.
	store, _ := p.selectionsLocked()
	if err := store.set(p.mustActiveID(t), "手选", "已删除"); err != nil {
		t.Fatal(err)
	}
	if err := p.StartActive(); err != nil {
		t.Fatal(err)
	}
	if now := running(); now != "东京" {
		t.Fatalf("missing node should fall back to the default: %q", now)
	}
	if saved := store.load(p.mustActiveID(t))["手选"]; saved != "东京" {
		t.Fatalf("saved choice did not follow the core: %q", saved)
	}
}

func (p *Profiles) mustActiveID(t *testing.T) string {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		t.Fatal(err)
	}
	return d.ActiveID
}
