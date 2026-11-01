package core

import (
	"strings"
	"testing"
)

func hopsOf(ips ...string) []RouteHop {
	hops := make([]RouteHop, len(ips))
	for i, ip := range ips {
		if ip == "*" {
			hops[i] = RouteHop{MS: -1}
			continue
		}
		hops[i] = RouteHop{IP: ip, MS: (i + 1) * 3}
	}
	return hops
}

func labels(route ForwardRoute) string {
	parts := []string{}
	for _, s := range route.Segments {
		parts = append(parts, s.Label+"/"+s.Kind)
	}
	return strings.Join(parts, " ")
}

func TestRouteSegmentsNameTheLine(t *testing.T) {
	gia := summarizeRoute(hopsOf("192.168.1.1", "100.64.0.1", "61.152.1.1", "*", "59.43.1.1", "59.43.2.2", "*", "203.0.113.9"), true)
	if got := labels(gia); got != "本地网络/local 城域网/carrier CN2 GIA/premium 入口/entry" {
		t.Fatalf("GIA segments = %s", got)
	}
	if !strings.HasPrefix(gia.Verdict, "去程走 CN2 GIA，8 跳到入口") || gia.Silent != 2 {
		t.Fatalf("GIA verdict = %q silent=%d", gia.Verdict, gia.Silent)
	}
	gt := summarizeRoute(hopsOf("192.168.1.1", "202.97.1.1", "59.43.1.1", "198.51.100.1", "203.0.113.9"), true)
	if got := labels(gt); got != "本地网络/local 电信 163/carrier CN2 GT/premium 国际段/intl 入口/entry" {
		t.Fatalf("GT segments = %s", got)
	}
	if !strings.HasPrefix(gt.Verdict, "去程走 CN2 GT") {
		t.Fatalf("GT verdict = %q", gt.Verdict)
	}
	abroad := summarizeRoute(hopsOf("10.0.0.1", "198.51.100.1", "198.51.100.2"), false)
	if got := labels(abroad); got != "本地网络/local 公网/carrier" || !strings.Contains(abroad.Verdict, "入口不回应探测") {
		t.Fatalf("abroad = %s / %q", got, abroad.Verdict)
	}
}
