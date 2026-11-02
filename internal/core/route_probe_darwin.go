//go:build darwin

package core

import (
	"net"
	"net/netip"
	"os"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/metacubex/mihomo/component/dialer"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

const (
	traceMaxTTL    = 40 // paths over 25 hops are ordinary through a carrier's internal network
	traceHopWait   = 900 * time.Millisecond
	traceGiveUp    = 10 // consecutive silent hops before stopping; a run of six is seen on live paths
	ipBoundIf      = 25
	icmpHeaderSize = 8
)

var traceID atomic.Uint32

// traceForward sends ICMP echoes with rising TTL from an unprivileged datagram
// socket. The socket is bound to the physical interface mihomo uses for DIRECT,
// so TUN does not capture the probes and the path is the real one.
func traceForward(target string) ForwardRoute {
	dst := net.ParseIP(target).To4()
	if dst == nil {
		return ForwardRoute{Error: "入口不是 IPv4 地址，暂不检测去程"}
	}
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_ICMP)
	if err != nil {
		return ForwardRoute{Error: "无法发送探测：" + err.Error()}
	}
	defer syscall.Close(fd)
	if ifi := physicalInterface(); ifi != nil {
		_ = syscall.SetsockoptInt(fd, syscall.IPPROTO_IP, ipBoundIf, ifi.Index)
	}
	tv := syscall.NsecToTimeval(int64(traceHopWait))
	_ = syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)
	var to syscall.SockaddrInet4
	copy(to.Addr[:], dst)
	id := int(traceID.Add(1)+uint32(os.Getpid())) & 0xffff
	hops := []RouteHop{}
	silent, answered := 0, false
	for ttl := 1; ttl <= traceMaxTTL; ttl++ {
		_ = syscall.SetsockoptInt(fd, syscall.IPPROTO_IP, syscall.IP_TTL, ttl)
		msg := icmp.Message{Type: ipv4.ICMPTypeEcho, Body: &icmp.Echo{ID: id, Seq: ttl, Data: []byte("rocketscience")}}
		packet, _ := msg.Marshal(nil)
		start := time.Now()
		if err := syscall.Sendto(fd, packet, 0, &to); err != nil {
			return ForwardRoute{Error: "无法发送探测：" + err.Error()}
		}
		hop, done := awaitHop(fd, dst, id, ttl, start)
		hops = append(hops, hop)
		if done {
			// A public address answering the first probe means TUN caught it.
			if ttl == 1 && !privateHop(netipAddr(target)) {
				return ForwardRoute{Error: "去程探测被 TUN 接管，无法测量"}
			}
			return summarizeRoute(hops, true)
		}
		if hop.MS < 0 {
			silent++
		} else {
			silent, answered = 0, true
		}
		if silent >= traceGiveUp {
			break
		}
	}
	if !answered {
		return ForwardRoute{Error: "去程探测没有任何回应"}
	}
	return summarizeRoute(hops, false)
}

// awaitHop reads until the reply to this probe arrives or the hop times out;
// replies to other probes are skipped.
func awaitHop(fd int, dst net.IP, id, seq int, start time.Time) (RouteHop, bool) {
	buf := make([]byte, 1500)
	for time.Since(start) < traceHopWait {
		n, from, err := syscall.Recvfrom(fd, buf, 0)
		if err != nil {
			break
		}
		peer, ok := from.(*syscall.SockaddrInet4)
		if !ok {
			continue
		}
		data := buf[:n]
		// macOS hands datagram ICMP sockets the IP header as well.
		if len(data) > 20 && data[0]>>4 == 4 {
			data = data[int(data[0]&0x0f)*4:]
		}
		msg, err := icmp.ParseMessage(1, data)
		if err != nil {
			continue
		}
		hop := RouteHop{IP: net.IP(peer.Addr[:]).String(), MS: int(time.Since(start).Milliseconds())}
		switch body := msg.Body.(type) {
		case *icmp.Echo:
			if msg.Type == ipv4.ICMPTypeEchoReply && body.Seq == seq && net.IP(peer.Addr[:]).Equal(dst) {
				return hop, true
			}
		case *icmp.TimeExceeded:
			if probeSeq(body.Data, dst) == seq {
				return hop, false
			}
		}
	}
	return RouteHop{MS: -1}, false
}

// probeSeq reads the echo sequence from the original packet quoted in a
// time-exceeded message, or -1 when it was not our probe to dst.
func probeSeq(quoted []byte, dst net.IP) int {
	if len(quoted) < 20 {
		return -1
	}
	ihl := int(quoted[0]&0x0f) * 4
	if len(quoted) < ihl+icmpHeaderSize || !net.IP(quoted[16:20]).Equal(dst) {
		return -1
	}
	return int(quoted[ihl+6])<<8 | int(quoted[ihl+7])
}

// physicalInterface is the interface mihomo binds DIRECT to; without one, the
// first Ethernet or Wi-Fi interface that is up with an IPv4 address.
func physicalInterface() *net.Interface {
	if name := dialer.DefaultInterface.Load(); name != "" {
		if ifi, err := net.InterfaceByName(name); err == nil {
			return ifi
		}
	}
	list, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for i := range list {
		ifi := &list[i]
		if ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagLoopback != 0 || !strings.HasPrefix(ifi.Name, "en") {
			continue
		}
		addrs, _ := ifi.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil && !ipnet.IP.IsLinkLocalUnicast() {
				return ifi
			}
		}
	}
	return nil
}

func netipAddr(s string) netip.Addr {
	ip, _ := netip.ParseAddr(s)
	return ip
}
