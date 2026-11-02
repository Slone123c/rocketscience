package core

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type cloudflareSite struct {
	city, country string
	lon, lat      float64
}

// Cloudflare's main sites by airport code. A site not listed here is reported
// as Cloudflare with no place rather than guessed.
var cloudflareSites = map[string]cloudflareSite{
	"HKG": {"Hong Kong", "HK", 114.17, 22.32}, "TPE": {"Taipei", "TW", 121.56, 25.03}, "NRT": {"Tokyo", "JP", 139.69, 35.69},
	"KIX": {"Osaka", "JP", 135.50, 34.69}, "ICN": {"Seoul", "KR", 126.98, 37.57}, "SIN": {"Singapore", "SG", 103.82, 1.35},
	"KUL": {"Kuala Lumpur", "MY", 101.69, 3.14}, "BKK": {"Bangkok", "TH", 100.50, 13.75}, "SGN": {"Ho Chi Minh City", "VN", 106.66, 10.76},
	"MNL": {"Manila", "PH", 120.98, 14.60}, "CGK": {"Jakarta", "ID", 106.85, -6.21}, "BOM": {"Mumbai", "IN", 72.88, 19.08},
	"DEL": {"Delhi", "IN", 77.21, 28.61}, "DXB": {"Dubai", "AE", 55.27, 25.20}, "SYD": {"Sydney", "AU", 151.21, -33.87},
	"MEL": {"Melbourne", "AU", 144.96, -37.81}, "LAX": {"Los Angeles", "US", -118.24, 34.05}, "SJC": {"San Jose", "US", -121.89, 37.34},
	"SEA": {"Seattle", "US", -122.33, 47.61}, "PHX": {"Phoenix", "US", -112.07, 33.45}, "DEN": {"Denver", "US", -104.99, 39.74},
	"DFW": {"Dallas", "US", -96.80, 32.78}, "ORD": {"Chicago", "US", -87.63, 41.88}, "ATL": {"Atlanta", "US", -84.39, 33.75},
	"MIA": {"Miami", "US", -80.19, 25.76}, "IAD": {"Ashburn", "US", -77.49, 39.04}, "EWR": {"Newark", "US", -74.17, 40.74},
	"YYZ": {"Toronto", "CA", -79.38, 43.65}, "YVR": {"Vancouver", "CA", -123.12, 49.28}, "GRU": {"Sao Paulo", "BR", -46.63, -23.55},
	"LHR": {"London", "GB", -0.13, 51.51}, "CDG": {"Paris", "FR", 2.35, 48.86}, "FRA": {"Frankfurt", "DE", 8.68, 50.11},
	"AMS": {"Amsterdam", "NL", 4.90, 52.37}, "MAD": {"Madrid", "ES", -3.70, 40.42}, "MXP": {"Milan", "IT", 9.19, 45.46},
	"ARN": {"Stockholm", "SE", 18.07, 59.33}, "WAW": {"Warsaw", "PL", 21.01, 52.23},
}

// cloudflareColo reads the site code from a /cdn-cgi/trace response.
func cloudflareColo(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if code, ok := strings.CutPrefix(line, "colo="); ok {
			return strings.TrimSpace(code)
		}
	}
	return ""
}

// cloudflareLocation places a site code; an unlisted code stays unknown but still names the network.
func cloudflareLocation(colo string) IPLocation {
	location := IPLocation{Network: cloudflareLabel, Source: "cloudflare"}
	if site, ok := cloudflareSites[colo]; ok {
		location.Known = true
		location.City, location.Country, location.Lon, location.Lat = site.city, site.country, site.lon, site.lat
	}
	return location
}

// locateCloudflareEntry asks the anycast address itself which site answered
// this Mac. An IP database only knows where the block is registered.
func locateCloudflareEntry(ip string) IPLocation {
	dial := directDialer(true)
	client := &http.Client{Timeout: 6 * time.Second, Transport: &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dial(ctx, network, net.JoinHostPort(ip, "443"))
		},
	}}
	response, err := client.Get("https://speed.cloudflare.com/cdn-cgi/trace")
	if err != nil {
		return IPLocation{Network: cloudflareLabel, Error: err.Error()}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4096))
	if err != nil {
		return IPLocation{Network: cloudflareLabel, Error: err.Error()}
	}
	return cloudflareLocation(cloudflareColo(string(body)))
}
