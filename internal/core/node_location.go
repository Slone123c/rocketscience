package core

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"time"

	"github.com/metacubex/mihomo/component/resolver"
	C "github.com/metacubex/mihomo/constant"
	mihomoLog "github.com/metacubex/mihomo/log"
	"github.com/metacubex/mihomo/tunnel"
)

// NodeLocation is where a proxy node really is, for the route map: Entry is
// its server (the address in the config, resolved and located), Exit is the
// public IP sites see when traffic leaves through it. They differ for relays,
// e.g. a Hong Kong entry that lands in Los Angeles. Forward is the measured
// path from this Mac to the entry. Pending means a lookup is still running;
// ask again shortly.
type NodeLocation struct {
	Entry   IPLocation   `json:"entry"`
	Exit    IPLocation   `json:"exit"`
	Forward ForwardRoute `json:"forward"`
	Pending bool         `json:"pending"`
}

const (
	nodeLocationTTL      = 30 * time.Minute
	nodeLocationRetry    = 2 * time.Minute
	nodeLocationParallel = 3
	maxNodeLocations     = 8
)

type nodeLocationEntry struct {
	value    NodeLocation
	at       time.Time
	inflight bool
}

var nodeLocations = struct {
	sync.Mutex
	cache map[string]*nodeLocationEntry
	slots chan struct{}
}{cache: map[string]*nodeLocationEntry{}, slots: make(chan struct{}, nodeLocationParallel)}

// NodeLocations returns what is known about each node right away and looks up
// the rest in the background. Only the busiest few nodes are worth a lookup:
// each exit check sends one small request through that node.
func (s *Service) NodeLocations(names []string) map[string]NodeLocation {
	s.mu.Lock()
	running := s.cmd != nil
	s.mu.Unlock()
	result := map[string]NodeLocation{}
	if !running {
		return result
	}
	if len(names) > maxNodeLocations {
		names = names[:maxNodeLocations]
	}
	nodeLocations.Lock()
	defer nodeLocations.Unlock()
	for _, name := range names {
		entry := nodeLocations.cache[name]
		if entry == nil {
			entry = &nodeLocationEntry{}
			nodeLocations.cache[name] = entry
		}
		ttl := nodeLocationTTL
		if !entry.value.Entry.Known || !entry.value.Exit.Known || !entry.value.Forward.Known {
			ttl = nodeLocationRetry
		}
		if !entry.inflight && (entry.at.IsZero() || time.Since(entry.at) > ttl) {
			entry.inflight = true
			go locateNode(name)
		}
		value := entry.value
		value.Pending = entry.inflight
		result[name] = value
	}
	return result
}

func locateNode(name string) {
	nodeLocations.slots <- struct{}{}
	defer func() { <-nodeLocations.slots }()
	value := NodeLocation{Entry: IPLocation{Error: "节点不存在"}, Exit: IPLocation{Error: "节点不存在"}, Forward: ForwardRoute{Error: "节点不存在"}}
	if proxy := findProxy(name); proxy != nil {
		var ip string
		ip, value.Entry = locateNodeEntry(proxy)
		value.Exit = locateNodeExit(proxy)
		value.Forward = ForwardRoute{Error: value.Entry.Error}
		if ip != "" {
			value.Forward = traceForward(ip)
			annotateForwardCities(&value.Forward)
		}
	}
	mihomoLog.Debugln("[节点位置] %s：入口 %s %s（%s），落地 %s %s（%s）", name, value.Entry.City, value.Entry.Country, value.Entry.Error, value.Exit.City, value.Exit.Country, value.Exit.Error)
	nodeLocations.Lock()
	if entry := nodeLocations.cache[name]; entry != nil {
		entry.value, entry.at, entry.inflight = value, time.Now(), false
	}
	nodeLocations.Unlock()
}

// findProxy looks in the config's proxies, then in proxy providers.
func findProxy(name string) C.Proxy {
	if proxy := tunnel.Proxies()[name]; proxy != nil {
		return proxy
	}
	for _, provider := range tunnel.Providers() {
		for _, proxy := range provider.Proxies() {
			if proxy.Name() == name {
				return proxy
			}
		}
	}
	return nil
}

// locateNodeEntry resolves the server address the way mihomo does for proxy
// servers, then locates that IP over a direct connection.
func locateNodeEntry(proxy C.Proxy) (string, IPLocation) {
	host, _, err := net.SplitHostPort(proxy.Addr())
	if err != nil || host == "" {
		return "", IPLocation{Error: "节点没有服务器地址"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ip := host
	if parsed := net.ParseIP(host); parsed == nil {
		addr, err := resolver.ResolveIPWithResolver(ctx, host, resolver.ProxyServerHostResolver)
		if err != nil {
			return "", IPLocation{Error: "无法解析节点地址：" + err.Error()}
		}
		ip = addr.Unmap().String()
	}
	client := &http.Client{Timeout: 6 * time.Second, Transport: &http.Transport{Proxy: nil, DialContext: directDialer(true)}}
	return ip, lookupIPLocation(client, ip)
}

// locateNodeExit asks for the caller's own location through the node itself.
func locateNodeExit(proxy C.Proxy) IPLocation {
	dial := func(ctx context.Context, _, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		number, err := strconv.ParseUint(port, 10, 16)
		if err != nil {
			return nil, err
		}
		return proxy.DialContext(ctx, &C.Metadata{NetWork: C.TCP, Host: host, DstPort: uint16(number)})
	}
	client := &http.Client{Timeout: 8 * time.Second, Transport: &http.Transport{Proxy: nil, DialContext: dial}}
	return lookupIPLocation(client, "")
}

// annotateForwardCities locates public hops that answered. Private addresses
// and silent hops stay unlabelled; a lookup that fails leaves that hop blank.
func annotateForwardCities(route *ForwardRoute) {
	if !route.Known {
		return
	}
	client := &http.Client{Timeout: 6 * time.Second, Transport: &http.Transport{Proxy: nil, DialContext: directDialer(true)}}
	seen := map[string]IPLocation{}
	for i := range route.Hops {
		hop := &route.Hops[i]
		if hop.MS < 0 || hop.IP == "" {
			continue
		}
		ip, err := netip.ParseAddr(hop.IP)
		if err != nil || privateHop(ip) {
			continue
		}
		location, ok := seen[hop.IP]
		if !ok {
			location = lookupIPLocation(client, hop.IP)
			seen[hop.IP] = location
		}
		if !location.Known || location.City == "" {
			continue
		}
		hop.City, hop.Country, hop.Lon, hop.Lat = location.City, location.Country, location.Lon, location.Lat
	}
}
