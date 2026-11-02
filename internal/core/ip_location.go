package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	C "github.com/metacubex/mihomo/constant"
	"github.com/metacubex/mihomo/tunnel"
)

// IPLocation is where this Mac's public IP is registered, for the route map's
// origin. It asks ipinfo.io (ipapi.co as a fallback) over a direct connection:
// through the proxy it would report the exit node instead.
type IPLocation struct {
	Known   bool    `json:"known"`
	City    string  `json:"city"`
	Region  string  `json:"region"`
	Country string  `json:"country"`
	Lon     float64 `json:"lon"`
	Lat     float64 `json:"lat"`
	Source  string  `json:"source"`
	// Network names an anycast network, whose address has no fixed place.
	Network string `json:"network,omitempty"`
	Error   string `json:"error"`
}

const ipLocationTTL = 10 * time.Minute

var ipLocationCache struct {
	sync.Mutex
	value IPLocation
	at    time.Time
}

// LocalIPLocation returns the public IP's city, cached for ten minutes.
func (s *Service) LocalIPLocation() IPLocation {
	ipLocationCache.Lock()
	defer ipLocationCache.Unlock()
	if time.Since(ipLocationCache.at) < ipLocationTTL {
		return ipLocationCache.value
	}
	s.mu.Lock()
	running := s.cmd != nil
	s.mu.Unlock()
	client := &http.Client{Timeout: 6 * time.Second, Transport: &http.Transport{
		Proxy:       nil,
		DialContext: directDialer(running),
	}}
	value := lookupIPLocation(client, "")
	ipLocationCache.value, ipLocationCache.at = value, time.Now()
	if !value.Known {
		// Retry a failure sooner, e.g. right after the network comes up.
		ipLocationCache.at = time.Now().Add(-ipLocationTTL + time.Minute)
	}
	return value
}

// directDialer bypasses the proxy. While the core runs, mihomo's DIRECT
// outbound binds the physical interface, so TUN does not capture the request.
func directDialer(running bool) func(context.Context, string, string) (net.Conn, error) {
	plain := (&net.Dialer{Timeout: 5 * time.Second}).DialContext
	if !running {
		return plain
	}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		direct := tunnel.Proxies()["DIRECT"]
		if direct == nil {
			return plain(ctx, network, address)
		}
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		number, err := strconv.ParseUint(port, 10, 16)
		if err != nil {
			return nil, err
		}
		return direct.DialContext(ctx, &C.Metadata{NetWork: C.TCP, Host: host, DstPort: uint16(number)})
	}
}

// lookupIPLocation locates ip, or the caller's own public IP when ip is empty.
func lookupIPLocation(client *http.Client, ip string) IPLocation {
	var failures []string
	for _, provider := range ipLocationProviders {
		value, err := provider.fetch(client, ip)
		if err == nil {
			value.Known, value.Source = true, provider.name
			return value
		}
		failures = append(failures, provider.name+"："+err.Error())
	}
	return IPLocation{Error: strings.Join(failures, "；")}
}

type ipLocationProvider struct {
	name string
	// url takes the IP to locate, or "" for the caller's own.
	url   func(ip string) string
	parse func([]byte) (IPLocation, error)
}

// HTTPS only: a looked-up address may be a node's entry, which must not cross
// the network in the clear.
var ipLocationProviders = []ipLocationProvider{
	{"ipinfo.io", func(ip string) string { return "https://ipinfo.io/" + pathIP(ip) + "json" }, parseIPInfo},
	{"ipapi.co", func(ip string) string { return "https://ipapi.co/" + pathIP(ip) + "json/" }, parseIPAPI},
}

func pathIP(ip string) string {
	if ip == "" {
		return ""
	}
	return ip + "/"
}

func (p ipLocationProvider) fetch(client *http.Client, ip string) (IPLocation, error) {
	request, err := http.NewRequest(http.MethodGet, p.url(ip), nil)
	if err != nil {
		return IPLocation{}, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return IPLocation{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return IPLocation{}, fmt.Errorf("HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return IPLocation{}, err
	}
	return p.parse(body)
}

// parseIPInfo reads ipinfo.io's {"city","region","country","loc":"lat,lon"}.
func parseIPInfo(body []byte) (IPLocation, error) {
	var raw struct{ City, Region, Country, Loc string }
	if err := json.Unmarshal(body, &raw); err != nil {
		return IPLocation{}, err
	}
	lat, lon, ok := strings.Cut(raw.Loc, ",")
	if !ok {
		return IPLocation{}, errors.New("响应缺少坐标")
	}
	latitude, err1 := strconv.ParseFloat(strings.TrimSpace(lat), 64)
	longitude, err2 := strconv.ParseFloat(strings.TrimSpace(lon), 64)
	if err1 != nil || err2 != nil {
		return IPLocation{}, errors.New("响应坐标无效")
	}
	return IPLocation{City: raw.City, Region: raw.Region, Country: raw.Country, Lon: longitude, Lat: latitude}, nil
}

// parseIPAPI reads ipapi.co's {"city","region","country_code","latitude","longitude"}.
func parseIPAPI(body []byte) (IPLocation, error) {
	var raw struct {
		City      string   `json:"city"`
		Region    string   `json:"region"`
		Country   string   `json:"country_code"`
		Latitude  *float64 `json:"latitude"`
		Longitude *float64 `json:"longitude"`
		Error     bool     `json:"error"`
		Reason    string   `json:"reason"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return IPLocation{}, err
	}
	if raw.Error || raw.Latitude == nil || raw.Longitude == nil {
		return IPLocation{}, fmt.Errorf("响应缺少坐标 %s", raw.Reason)
	}
	return IPLocation{City: raw.City, Region: raw.Region, Country: raw.Country, Lon: *raw.Longitude, Lat: *raw.Latitude}, nil
}
