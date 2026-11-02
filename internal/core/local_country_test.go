package core

import (
	"net/netip"
	"testing"
)

func TestCountryOfRecord(t *testing.T) {
	cases := []struct {
		record any
		want   string
	}{
		{"cn", "CN"},
		{[]any{"us", "cloudflare"}, "US"},
		{[]any{"cloudflare", "jp"}, "JP"},
		{map[string]any{"country": map[string]any{"iso_code": "DE"}}, "DE"},
		{map[string]any{"continent": map[string]any{"code": "AS"}}, ""},
		{"private", ""},
		{nil, ""},
	}
	for _, c := range cases {
		if got := countryOfRecord(c.record); got != c.want {
			t.Fatalf("countryOfRecord(%v) = %q, want %q", c.record, got, c.want)
		}
	}
}

func TestMissingCountryDatabaseLeavesHopsBlank(t *testing.T) {
	lookup, closeDB := openLocalCountryDB(t.TempDir())
	defer closeDB()
	if got := lookup(netip.MustParseAddr("8.8.8.8")); got != "" {
		t.Fatalf("lookup = %q", got)
	}
}

func TestAnnotatingHopsNeedsNoNetwork(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_DATA_DIR", t.TempDir())
	route := summarizeRoute(hopsOf("192.168.1.1", "*", "8.8.8.8"), true)
	annotateForwardCountries(&route)
	for _, hop := range route.Hops {
		if hop.Country != "" {
			t.Fatalf("hop = %+v", hop)
		}
	}
}
