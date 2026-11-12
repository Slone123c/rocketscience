//go:build darwin

package core

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/metacubex/mihomo/component/dialer"
	"github.com/metacubex/mihomo/listener"
	"github.com/metacubex/mihomo/tunnel"
	"github.com/metacubex/mihomo/tunnel/statistic"
	"rocketscience/internal/tunhelper"
)

func tunServiceLogPath() string { return tunhelper.ServiceLog }

// diagnoseTun checks a TUN session the way one would by hand: does traffic
// route into our device, does another TUN own the routes, which interface
// mihomo dials out of, and do DNS and a TCP connection get through. It logs
// everything and warns in the status when traffic bypasses the device.
func (s *Service) diagnoseTun(device string, done <-chan struct{}) {
	select {
	case <-done:
		return
	case <-time.After(time.Second):
	}
	ours := map[string]bool{}
	if iface, err := net.InterfaceByName(device); err != nil {
		tunDebugf("自检：找不到虚拟网卡 %s：%v", device, err)
	} else {
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ip, _, err := net.ParseCIDR(addr.String()); err == nil {
				ours[ip.String()] = true
			}
		}
		tunDebugf("自检：虚拟网卡 %s flags=%s mtu=%d 地址=%v", device, iface.Flags, iface.MTU, addrs)
	}
	var conflicts []string
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		if iface.Name == device || !strings.HasPrefix(iface.Name, "utun") {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ip, _, err := net.ParseCIDR(addr.String()); err == nil && ip.To4() != nil {
				tunDebugf("自检：其他 utun %s 地址 %s", iface.Name, addr)
				if ours[ip.String()] {
					conflicts = append(conflicts, iface.Name)
				}
			}
		}
	}
	dns := systemDNSServer()
	routes := map[string]string{}
	for _, target := range []string{"1.1.1.1", "8.8.8.8"} {
		via := routeInterface(target)
		routes[target] = via
		tunDebugf("自检：%s 的路由出口是 %s", target, via)
	}
	if dns != "" {
		via := routeInterface(dns)
		routes[dns] = via
		tunDebugf("自检：%s 的路由出口是 %s", dns, via)
		if via != device && !dnsRouteIsExpectedElsewhere(dns) && !isLocalOrPrivateIP(dns) {
			// Non-CGNAT system DNS that skips our TUN is unusual; still do not
			// warn from DNS alone when public probes enter the device.
			tunDebugf("自检：系统 DNS %s 未走本机 TUN（出口 %s），公网探测结果另计", dns, via)
		}
	}
	bypassed := probeBypassesTUN(device, routes)
	conf := listener.GetTunConf()
	tunDebugf("自检：mihomo TUN enable=%v stack=%s fd=%d dns-hijack=%v auto-detect-interface=%v；出站网卡=%q；模式=%s；系统 DNS=%s",
		conf.Enable, conf.Stack, conf.FileDescriptor, conf.DNSHijack, conf.AutoDetectInterface, dialer.DefaultInterface.Load(), tunnel.Mode(), dns)
	before := connectionCount()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	started := time.Now()
	addrs, err := net.DefaultResolver.LookupHost(ctx, "www.apple.com")
	cancel()
	tunDebugf("自检：解析 www.apple.com → %v，错误=%v，用时 %s", addrs, err, time.Since(started).Round(time.Millisecond))
	started = time.Now()
	conn, err := net.DialTimeout("tcp", "1.1.1.1:443", 4*time.Second)
	elapsed := time.Since(started).Round(time.Millisecond)
	during := before
	if err == nil {
		// The TUN stack accepts at once; mihomo tracks the connection only
		// after the rule match, so count while it is still open.
		time.Sleep(300 * time.Millisecond)
		during = connectionCount()
		conn.Close()
	}
	tunDebugf("自检：TCP 连接 1.1.1.1:443 错误=%v，用时 %s；mihomo 连接数 %d → %d（增加说明流量进入了 TUN）", err, elapsed, before, during)

	warning := ""
	switch {
	case len(conflicts) > 0:
		warning = fmt.Sprintf("TUN 已启动，但 %s 使用了相同的地址，可能是其他 VPN 或代理应用的 TUN，请先关闭它", strings.Join(conflicts, "、"))
	case bypassed != "":
		warning = fmt.Sprintf("TUN 已启动，但系统流量走的是 %s 而不是 %s，可能被其他 VPN 或 TUN 占用", bypassed, device)
	}
	if warning == "" {
		tunDebugf("自检：完成，未发现路由问题")
		return
	}
	tunDebugf("自检：%s", warning)
	s.mu.Lock()
	if s.done == done && s.proxyWarning == "" {
		s.proxyWarning = warning
	}
	s.mu.Unlock()
}

func connectionCount() int {
	count := 0
	statistic.DefaultManager.Range(func(statistic.Tracker) bool { count++; return true })
	return count
}

func routeInterface(target string) string {
	output, err := exec.Command("/sbin/route", "-n", "get", target).Output()
	if err != nil {
		return "（查询失败：" + err.Error() + "）"
	}
	for _, line := range strings.Split(string(output), "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "interface:"); ok {
			return strings.TrimSpace(value)
		}
	}
	return "（无）"
}

func isLocalOrPrivateIP(raw string) bool {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

func systemDNSServer() string {
	output, err := exec.Command("/usr/sbin/scutil", "--dns").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(output), "\n") {
		if _, value, ok := strings.Cut(line, "nameserver[0] :"); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
