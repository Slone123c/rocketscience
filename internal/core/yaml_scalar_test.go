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
