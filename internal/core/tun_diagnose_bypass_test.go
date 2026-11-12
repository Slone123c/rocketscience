package core

import "testing"

func TestProbeBypassesTUNIgnoresMagicDNS(t *testing.T) {
	routes := map[string]string{
		"1.1.1.1":         "utun10",
		"8.8.8.8":         "utun10",
		"100.100.100.100": "utun7",
	}
	if got := probeBypassesTUN("utun10", routes); got != "" {
		t.Fatalf("Tailscale MagicDNS looked like a bypass: %q", got)
	}
	routes["1.1.1.1"] = "utun7"
	if got := probeBypassesTUN("utun10", routes); got != "utun7" {
		t.Fatalf("public probe bypass missed: %q", got)
	}
}

func TestDNSRouteIsExpectedElsewhere(t *testing.T) {
	for _, dns := range []string{"100.100.100.100", "100.64.0.1", "100.127.255.255"} {
		if !dnsRouteIsExpectedElsewhere(dns) {
			t.Fatalf("%s should be treated as other-VPN DNS", dns)
		}
	}
	for _, dns := range []string{"223.6.6.6", "1.1.1.1", "8.8.8.8", "192.168.1.1"} {
		if dnsRouteIsExpectedElsewhere(dns) {
			t.Fatalf("%s should not be exempt", dns)
		}
	}
}
