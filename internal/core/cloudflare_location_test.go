package core

import (
	"net/netip"
	"testing"
)

func TestCloudflareSiteFromTrace(t *testing.T) {
	colo := cloudflareColo("fl=1\nip=203.0.113.1\ncolo=SIN\nloc=CN\n")
	if colo != "SIN" {
		t.Fatalf("colo = %q", colo)
	}
	if got := cloudflareLocation(colo); !got.Known || got.City != "Singapore" || got.Country != "SG" || got.Network != "Cloudflare" {
		t.Fatalf("location = %+v", got)
	}
	if got := cloudflareLocation("XXX"); got.Known || got.City != "" || got.Network != "Cloudflare" {
		t.Fatalf("unlisted site = %+v", got)
	}
}

func TestIsCloudflare(t *testing.T) {
	for ip, want := range map[string]bool{"162.159.13.2": true, "172.69.117.39": true, "104.16.1.1": true, "2.59.151.229": false, "202.97.41.130": false} {
		if got := isCloudflare(netip.MustParseAddr(ip)); got != want {
			t.Fatalf("isCloudflare(%s) = %v", ip, got)
		}
	}
}
