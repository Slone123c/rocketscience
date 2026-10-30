// Package tunhelper is the protocol between the app and rs-tun-helper.
//
// macOS only lets root create a utun device and change routes, while mihomo runs
// inside the unprivileged app. The app listens on a private Unix socket and
// starts the helper with administrator privileges. The helper connects, reads
// one Request, creates the device with its addresses and routes, and answers
// with one Response that carries the device's descriptor (SCM_RIGHTS). It then
// waits until the app closes the connection, which also happens when the app
// exits or crashes, and removes what it set up. The device itself disappears
// once the last descriptor is closed, taking its routes with it.
//
// So that only installation asks for a password, the helper is normally
// installed once as a launchd daemon (the "service mode" of other macOS proxy
// apps) and listens on ServiceSocket. The app then connects there itself; the
// one-shot mode above remains as a fallback.
package tunhelper

import (
	"errors"
	"fmt"
	"net/netip"
	"regexp"
)

// Request describes the device mihomo's configuration asks for. Prefixes are
// strings so the JSON stays readable in logs.
type Request struct {
	Name         string   `json:"name"`
	MTU          uint32   `json:"mtu"`
	Inet4Address []string `json:"inet4Address"`
	Inet6Address []string `json:"inet6Address"`
	AutoRoute    bool     `json:"autoRoute"`
	DNSMode      string   `json:"dnsMode"`
	RecvMsgX     bool     `json:"recvMsgX"`
	RouteAddress []string `json:"routeAddress"`
	RouteExclude []string `json:"routeExclude"`
}

// Response names the device that was created, or explains why it was not.
type Response struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

const (
	ServiceLabel  = "com.rocketscience.tun-helper"
	ServicePath   = "/Library/PrivilegedHelperTools/" + ServiceLabel
	ServicePlist  = "/Library/LaunchDaemons/" + ServiceLabel + ".plist"
	ServiceSocket = "/var/run/" + ServiceLabel + ".sock"
	ServiceLog    = "/Library/Logs/" + ServiceLabel + ".log"
)

const maxPrefixes = 4096

var utunName = regexp.MustCompile(`^utun[0-9]{1,4}$`)

// Validate rejects requests the helper must not act on as root.
func (r Request) Validate() error {
	if r.Name != "" && !utunName.MatchString(r.Name) {
		return fmt.Errorf("虚拟网卡名称无效：%q", r.Name)
	}
	if r.MTU < 576 || r.MTU > 65535 {
		return errors.New("TUN MTU 必须为 576 到 65535 之间的整数")
	}
	if len(r.Inet4Address) == 0 {
		return errors.New("虚拟网卡缺少 IPv4 地址")
	}
	switch r.DNSMode {
	case "", "disabled", "native", "hijack":
	default:
		return errors.New("TUN DNS 模式无效")
	}
	for _, group := range [][]string{r.Inet4Address, r.Inet6Address, r.RouteAddress, r.RouteExclude} {
		if len(group) > maxPrefixes {
			return errors.New("TUN 路由条目过多")
		}
		if _, err := ParsePrefixes(group); err != nil {
			return err
		}
	}
	return nil
}

func ParsePrefixes(values []string) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return nil, fmt.Errorf("TUN 地址无效：%q", value)
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}

func PrefixStrings(prefixes []netip.Prefix) []string {
	values := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		values = append(values, prefix.String())
	}
	return values
}
