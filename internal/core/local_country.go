package core

import (
	"net"
	"net/netip"
	"path/filepath"
	"strings"

	"github.com/oschwald/maxminddb-golang"
)

// openLocalCountryDB opens the GeoIP file the core already downloaded. mihomo's own
// reader exits the process when the file is missing, so this one opens it separately.
// Without a usable file every lookup is blank.
func openLocalCountryDB(dir string) (lookup func(netip.Addr) string, closeDB func()) {
	for _, name := range []string{"geoip.metadb", "country.mmdb"} {
		reader, err := maxminddb.Open(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		return func(ip netip.Addr) string {
			var record any
			if err := reader.Lookup(net.IP(ip.AsSlice()), &record); err != nil {
				return ""
			}
			return countryOfRecord(record)
		}, func() { _ = reader.Close() }
	}
	return func(netip.Addr) string { return "" }, func() {}
}

// countryOfRecord reads the three record shapes these databases use: "cn" or
// ["cn", "..."] (metadb), and {"country": {"iso_code": "CN"}} (MaxMind).
func countryOfRecord(record any) string {
	switch value := record.(type) {
	case string:
		if len(value) == 2 {
			return strings.ToUpper(value)
		}
	case []any:
		for _, item := range value {
			if code := countryOfRecord(item); code != "" {
				return code
			}
		}
	case map[string]any:
		if country, ok := value["country"].(map[string]any); ok {
			return countryOfRecord(country["iso_code"])
		}
	}
	return ""
}
