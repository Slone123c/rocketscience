package core

import (
	"net"
	"strings"
)

// probeBypassesTUN reports the first interface that carries a public probe away
// from our TUN device. System DNS is not part of this decision: MagicDNS /
// CGNAT servers (Tailscale 100.100.100.100 and similar) are meant to stay on
// another utun while 1.1.1.1 / 8.8.8.8 still enter ours.
func probeBypassesTUN(device string, routes map[string]string) string {
	for _, target := range []string{"1.1.1.1", "8.8.8.8"} {
		via := routes[target]
		if via == "" || via == device || strings.HasPrefix(via, "（") {
			continue
		}
		return via
	}
	return ""
}

// dnsRouteIsExpectedElsewhere is true for DNS that other VPNs own on purpose
// (CGNAT / shared address space), so it must not count as "TUN bypassed".
func dnsRouteIsExpectedElsewhere(dns string) bool {
	ip := net.ParseIP(strings.TrimSpace(dns))
	if ip == nil {
		return false
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	// RFC 6598 100.64.0.0/10 — Tailscale, some carrier CGNAT, MagicDNS.
	return ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127
}
