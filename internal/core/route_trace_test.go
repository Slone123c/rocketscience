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
	if gia.Silent != 2 {
		t.Fatalf("GIA silent = %d", gia.Silent)
	}
	gt := summarizeRoute(hopsOf("192.168.1.1", "202.97.1.1", "59.43.1.1", "198.51.100.1", "203.0.113.9"), true)
	if got := labels(gt); got != "本地网络/local 电信 163/carrier CN2 GT/premium 国际段/intl 入口/entry" {
		t.Fatalf("GT segments = %s", got)
	}
	abroad := summarizeRoute(hopsOf("10.0.0.1", "198.51.100.1", "198.51.100.2"), false)
	if got := labels(abroad); got != "本地网络/local 公网/carrier 第 3 跳仍未到入口/unreached" || abroad.Reached {
		t.Fatalf("abroad = %s reached=%v", got, abroad.Reached)
	}
}

// The shape of a real trace that gave up: six silent hops after a carrier's internal network.
func TestUnfinishedTraceEndsAtTheLastAnswer(t *testing.T) {
	route := summarizeRoute(hopsOf("192.168.2.1", "*", "121.207.252.97", "10.6.13.14", "*", "*", "*"), false)
	if got := labels(route); got != "本地网络/local 公网/carrier 第 4 跳后无回应/unreached" {
		t.Fatalf("segments = %s", got)
	}
	tail := route.Segments[len(route.Segments)-1]
	if tail.Hops != 3 || tail.MS != 0 {
		t.Fatalf("tail = %+v", tail)
	}
	total := 0
	for _, s := range route.Segments {
		total += s.Hops
	}
	if total != len(route.Hops) {
		t.Fatalf("segments cover %d of %d hops", total, len(route.Hops))
	}
}

func TestCloudflareHopsAreNamed(t *testing.T) {
	route := summarizeRoute(hopsOf("192.168.2.1", "121.207.252.97", "172.69.117.39", "*", "162.159.13.2"), true)
	if got := labels(route); got != "本地网络/local 城域网/carrier Cloudflare/carrier 入口/entry" {
		t.Fatalf("segments = %s", got)
	}
}
