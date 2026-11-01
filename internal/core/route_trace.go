package core

import (
	"fmt"
	"net/netip"
	"strings"
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
// Kind is local, carrier, premium, intl or entry.
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
	Verdict  string         `json:"verdict"`
	Error    string         `json:"error"`
}

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

func privateHop(ip netip.Addr) bool {
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || netip.MustParsePrefix("100.64.0.0/10").Contains(ip)
}

// summarizeRoute labels each hop, then groups them. Hops the table does not
// know are named by where they sit: before the first backbone is the local
// operator's metro network, after the last one is the international leg.
func summarizeRoute(hops []RouteHop, reached bool) ForwardRoute {
	route := ForwardRoute{Known: true, Reached: reached, Hops: hops}
	n := len(hops)
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
	for i := range hops {
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
	route.Verdict = routeVerdict(route)
	return route
}

func routeVerdict(route ForwardRoute) string {
	seen := map[string]bool{}
	for _, s := range route.Segments {
		seen[s.Label] = true
	}
	var line string
	switch {
	case seen["CN2"] && seen["电信 163"]:
		line = "去程走 CN2 GT（先经电信 163）"
	case seen["CN2"]:
		line = "去程走 CN2 GIA"
	case seen["联通 9929"]:
		line = "去程走联通 9929"
	case seen["移动 CMI"]:
		line = "去程走移动 CMI"
	case seen["电信 163"]:
		line = "去程走电信 163 普通线路"
	case seen["联通 4837"]:
		line = "去程走联通 4837 普通线路"
	case seen["移动 CMNET"]:
		line = "去程走移动 CMNET"
	default:
		line = "去程未经过国内骨干网"
	}
	// Rename CN2 to the line it turned out to be, so the strip and the verdict agree.
	for i := range route.Segments {
		if route.Segments[i].Label == "CN2" {
			route.Segments[i].Label = strings.TrimPrefix(strings.SplitN(line, "（", 2)[0], "去程走 ")
		}
	}
	if !route.Reached {
		return fmt.Sprintf("%s；入口不回应探测，只测到第 %d 跳", line, len(route.Hops))
	}
	return fmt.Sprintf("%s，%d 跳到入口，%d ms", line, len(route.Hops), route.Hops[len(route.Hops)-1].MS)
}
