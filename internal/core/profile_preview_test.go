package core

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestProfilePreviewWithoutCore(t *testing.T) {
	p := NewProfiles(t.TempDir())
	content := `proxies:
  - {name: 东京, type: ss, server: private.example, password: private-secret, udp: true}
  - {name: 独立节点, type: trojan, password: hidden}
proxy-groups:
  - {name: 自动, type: url-test, proxies: [东京]}
  - {name: 手选, type: select, proxies: [自动, DIRECT, REJECT]}
  - {name: 提供者, type: select, use: [remote]}
proxy-providers:
  remote: {type: http, url: 'https://private.example/token'}
`
	if err := p.write(profileData{ActiveID: "active", Profiles: []Profile{{ID: "other", Content: "bad: ["}, {ID: "active", Content: content}}}); err != nil {
		t.Fatal(err)
	}
	groups, err := p.PreviewGroups()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 4 || groups[0].Name != "自动" || groups[0].Type != "URLTest" || groups[0].Nodes[0].Type != "ss" || groups[0].Nodes[0].UDP == nil || !*groups[0].Nodes[0].UDP {
		t.Fatalf("preview: %+v", groups)
	}
	if !reflect.DeepEqual(groups[1].All, []string{"自动", "DIRECT", "REJECT"}) || groups[1].Nodes[0].Type != "URLTest" || len(groups[2].All) != 0 || groups[3].All[0] != "独立节点" {
		t.Fatalf("members: %+v", groups)
	}
	for _, group := range groups {
		// A Selector shows the node it uses on the next start (mihomo's default is
		// its first member); automatic groups decide only after the core tests them.
		want := map[string]string{"手选": "自动"}[group.Name]
		if group.Now != want {
			t.Fatalf("preview selection for %s: %q, want %q", group.Name, group.Now, want)
		}
		for _, node := range group.Nodes {
			if node.Delay != nil || node.DelayAt != "" {
				t.Fatal("preview must not invent measurements")
			}
		}
	}
	encoded, _ := json.Marshal(groups)
	for _, secret := range []string{"private.example", "private-secret", "hidden", "https://"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatal("preview leaked configuration credentials")
		}
	}
}

func TestProfilePreviewAliasesAndInvalidConfig(t *testing.T) {
	groups, err := previewProfileGroups("proxies:\n- &node {name: a, type: vmess}\nproxy-groups:\n- {name: pick, type: select, proxies: [a]}\n")
	if err != nil || len(groups) != 1 || groups[0].Nodes[0].Type != "vmess" {
		t.Fatalf("%+v %v", groups, err)
	}
	if _, err = previewProfileGroups("proxies: [private-secret"); err == nil || strings.Contains(err.Error(), "private-secret") {
		t.Fatal("invalid YAML needs a redacted error")
	}
	p := NewProfiles(t.TempDir())
	groups, err = p.PreviewGroups()
	if err != nil || len(groups) != 0 {
		t.Fatalf("empty profile: %+v %v", groups, err)
	}
}
