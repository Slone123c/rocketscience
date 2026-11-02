package core

import (
	"fmt"
	"net/netip"
)

// RouteHop is one traceroute hop; MS is -1 for a hop that never answered.
// City is set only for a public hop whose city could be located.
type RouteHop struct {
	IP      string  `json:"ip"`
	MS      int     `json:"ms"`
	City    string  `json:"city,omitempty"`
	Country string  `json:"country,omitempty"`
	Lon     float64 `json:"lon"`
	Lat     float64 `json:"lat"`
}

// RouteSegment groups consecutive hops on the same network.
// Kind is local, carrier, premium, intl, entry or unreached (the end of a trace
// that stopped short; Hops counts the silent hops it stopped on).
type RouteSegment struct {
	Label string `json:"label"`
	Kind  string `json:"kind"`
	Hops  int    `json:"hops"`
	MS    int    `json:"ms"`
}

// ForwardRoute is the measured outbound path from this Mac to a node's entry server.
type ForwardRoute struct {
	Known    bool           `json:"known"`
	Reached  bool           `json:"reached"`
	Hops     []RouteHop     `json:"hops"`
	Segments []RouteSegment `json:"segments"`
	Silent   int            `json:"silent"`
	Error    string         `json:"error"`
}

const cloudflareLabel = "Cloudflare"

type backbone struct {
	prefix  netip.Prefix
	label   string
	premium bool
}

// Mainland backbones and paid lines, by address block. Each is what people
// mean by a line type; anything else is located only by its position on the path.
var backbones = func() []backbone {
	list := []struct {
		cidr, label string
		premium     bool
	}{
		{"202.97.0.0/16", "电信 163", false},
		{"59.43.0.0/16", "CN2", true},
		{"219.158.0.0/16", "联通 4837", false},
		{"218.105.0.0/16", "联通 9929", true},
		{"210.51.0.0/16", "联通 9929", true},
		{"221.183.0.0/16", "移动 CMNET", false},
		{"223.118.0.0/15", "移动 CMI", true},
		{"223.120.0.0/15", "移动 CMI", true},
		{"183.91.56.0/22", "电信香港", false},
	}
	// Cloudflare's published IPv4 ranges: https://www.cloudflare.com/ips-v4
	for _, cidr := range []string{
		"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22", "141.101.64.0/18",
		"108.162.192.0/18", "190.93.240.0/20", "188.114.96.0/20", "197.234.240.0/22", "198.41.128.0/17",
		"162.158.0.0/15", "104.16.0.0/13", "104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22",
	} {
		list = append(list, struct {
			cidr, label string
			premium     bool
		}{cidr, cloudflareLabel, false})
	}
	out := make([]backbone, 0, len(list))
	for _, item := range list {
		out = append(out, backbone{netip.MustParsePrefix(item.cidr), item.label, item.premium})
	}
	return out
}()

func backboneOf(ip netip.Addr) (backbone, bool) {
	for _, b := range backbones {
		if b.prefix.Contains(ip) {
			return b, true
		}
	}
	return backbone{}, false
}

// isCloudflare reports an anycast address: whichever site answers depends on
// the caller, so its registered city says nothing about where traffic goes.
func isCloudflare(ip netip.Addr) bool {
	b, ok := backboneOf(ip.Unmap())
	return ok && b.label == cloudflareLabel
}

func privateHop(ip netip.Addr) bool {
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || netip.MustParsePrefix("100.64.0.0/10").Contains(ip)
}

// summarizeRoute labels each hop, then groups them. Hops the table does not
// know are named by where they sit: before the first backbone is the local
// operator's metro network, after the last one is the international leg.
func summarizeRoute(hops []RouteHop, reached bool) ForwardRoute {
	route := ForwardRoute{Known: true, Reached: reached, Hops: hops}
	n := len(hops)
	// Silent hops at the end of an unfinished trace were never placed on any
	// network, so they go to the closing segment rather than the one before them.
	measured := n
	if !reached {
		for measured > 0 && hops[measured-1].MS < 0 {
			measured--
		}
	}
	labels := make([]backbone, n)
	known := make([]bool, n)
	first, last, public := -1, -1, false
	for i, hop := range hops {
		if hop.MS < 0 {
			route.Silent++
			continue
		}
		ip, err := netip.ParseAddr(hop.IP)
		if err != nil {
			continue
		}
		if b, ok := backboneOf(ip); ok {
			labels[i], known[i] = b, true
			if first < 0 {
				first = i
			}
			last = i
		}
		if !public && privateHop(ip) {
			labels[i], known[i] = backbone{label: "本地网络"}, true
		} else {
			public = true
		}
	}
	kindOf := func(i int) (string, string) {
		if reached && i == n-1 {
			return "入口", "entry"
		}
		if known[i] {
			if labels[i].label == "本地网络" {
				return "本地网络", "local"
			}
			if labels[i].premium {
				return labels[i].label, "premium"
			}
			return labels[i].label, "carrier"
		}
		switch {
		case first < 0:
			return "公网", "carrier"
		case i < first:
			return "城域网", "carrier"
		case i > last:
			return "国际段", "intl"
		}
		return "", ""
	}
	// Silent and unknown hops in the middle join the segment before them.
	for i := range hops[:measured] {
		label, kind := kindOf(i)
		if hops[i].MS < 0 || label == "" {
			if len(route.Segments) > 0 && !(reached && i == n-1) {
				route.Segments[len(route.Segments)-1].Hops++
				continue
			}
			if label == "" {
				label, kind = "城域网", "carrier"
			}
		}
		if s := len(route.Segments); s > 0 && route.Segments[s-1].Label == label {
			route.Segments[s-1].Hops++
			if hops[i].MS >= 0 {
				route.Segments[s-1].MS = hops[i].MS
			}
			continue
		}
		route.Segments = append(route.Segments, RouteSegment{Label: label, Kind: kind, Hops: 1, MS: max(hops[i].MS, 0)})
	}
	nameCN2Line(route.Segments)
	if !reached {
		label := fmt.Sprintf("第 %d 跳仍未到入口", measured)
		if measured < n {
			label = fmt.Sprintf("第 %d 跳后无回应", measured)
		}
		route.Segments = append(route.Segments, RouteSegment{Label: label, Kind: "unreached", Hops: n - measured})
	}
	return route
}

// nameCN2Line renames CN2 to the line it turned out to be: a path that also
// crossed 163 is GT, otherwise it is GIA.
func nameCN2Line(segments []RouteSegment) {
	line := "CN2 GIA"
	for _, s := range segments {
		if s.Label == "电信 163" {
			line = "CN2 GT"
		}
	}
	for i := range segments {
		if segments[i].Label == "CN2" {
			segments[i].Label = line
		}
	}
}
