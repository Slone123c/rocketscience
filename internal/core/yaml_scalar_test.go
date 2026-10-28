package core

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// CFG-01: hex-looking strings must survive the runtime config round trip.
func TestRuntimeConfigPreservesNumericLookingScalars(t *testing.T) {
	src := "proxies:\n- name: a\n  type: vless\n  server: 1.2.3.4\n  port: 443\n  uuid: 12345678\n  reality-opts:\n    public-key: k\n    short-id: %s\nproxy-groups: []\nrules: []\n"
	for _, id := range []string{"01234567", "1234e5", "0x12", "12345678", "0b11"} {
		out, err := runtimeConfigWithOptions(strings.Replace(src, "%s", id, 1), 7890, 9090, "s", "rule", LANSettings{}, false, CoreOptions{})
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Proxies []struct {
				Reality struct {
					ShortID yaml.Node `yaml:"short-id"`
				} `yaml:"reality-opts"`
			} `yaml:"proxies"`
		}
		if err = yaml.Unmarshal(out, &doc); err != nil {
			t.Fatal(err)
		}
		if got := doc.Proxies[0].Reality.ShortID.Value; got != id {
			t.Errorf("short-id %q became %q\n%s", id, got, out)
		}
	}
}

// CFG-06: order-sensitive nested mappings (e.g. dns.nameserver-policy, hosts, fallback-filter)
// must preserve their original key order across runtimeConfigWithOptions and DNS overrides.
func TestRuntimeConfigPreservesNestedMappingKeyOrder(t *testing.T) {
	src := "dns:\n  enable: true\n  nameserver-policy:\n    geosite:cn: 223.5.5.5\n    +.a.com: 114.114.114.114\n    geosite:geolocation-!cn: 1.1.1.1\nhosts:\n  z.example.com: 10.0.0.1\n  a.example.com: 10.0.0.2\nrules: []\n"
	opts := CoreOptions{DNS: &DNSOptions{Enabled: ptr(true), EnhancedMode: ptr("fake-ip")}}
	out, err := runtimeConfigWithOptions(src, 7890, 9090, "s", "rule", LANSettings{}, false, opts)
	if err != nil {
		t.Fatal(err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	root := doc.Content[0]
	dnsNode := yamlField(root, "dns")
	policyNode := yamlField(dnsNode, "nameserver-policy")
	if policyNode == nil || len(policyNode.Content) != 6 {
		t.Fatalf("unexpected nameserver-policy node: %+v\n%s", policyNode, out)
	}
	gotPolicyKeys := []string{policyNode.Content[0].Value, policyNode.Content[2].Value, policyNode.Content[4].Value}
	wantPolicyKeys := []string{"geosite:cn", "+.a.com", "geosite:geolocation-!cn"}
	if strings.Join(gotPolicyKeys, ",") != strings.Join(wantPolicyKeys, ",") {
		t.Fatalf("nameserver-policy key order changed: got %v, want %v\n%s", gotPolicyKeys, wantPolicyKeys, out)
	}
	hostsNode := yamlField(root, "hosts")
	if hostsNode == nil || len(hostsNode.Content) != 4 {
		t.Fatalf("unexpected hosts node: %+v\n%s", hostsNode, out)
	}
	gotHostKeys := []string{hostsNode.Content[0].Value, hostsNode.Content[2].Value}
	wantHostKeys := []string{"z.example.com", "a.example.com"}
	if strings.Join(gotHostKeys, ",") != strings.Join(wantHostKeys, ",") {
		t.Fatalf("hosts key order changed: got %v, want %v\n%s", gotHostKeys, wantHostKeys, out)
	}

	// Also verify YAML anchors and << merges inside preserved subtrees (e.g. proxy-providers).
	anchored := "p: &base\n  type: http\n  interval: 3600\nproxy-providers:\n  z-prov:\n    <<: *base\n    url: https://example.com/z\n  a-prov:\n    <<: *base\n    url: https://example.com/a\nrules: []\n"
	out2, err := runtimeConfigWithOptions(anchored, 7890, 9090, "s", "rule", LANSettings{}, false, opts)
	if err != nil {
		t.Fatal(err)
	}
	var doc2 yaml.Node
	if err := yaml.Unmarshal(out2, &doc2); err != nil {
		t.Fatal(err)
	}
	ppNode := yamlField(doc2.Content[0], "proxy-providers")
	if ppNode == nil || len(ppNode.Content) != 4 || ppNode.Content[0].Value != "z-prov" || ppNode.Content[2].Value != "a-prov" {
		t.Fatalf("proxy-providers order/merge broken:\n%s", out2)
	}
	if yamlField(ppNode.Content[1], "interval") == nil || yamlField(ppNode.Content[1], "interval").Value != "3600" {
		t.Fatalf("<< merge not expanded in preserved subtree:\n%s", out2)
	}
}
