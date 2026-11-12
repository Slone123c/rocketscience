//go:build darwin && roottest

package main

import (
	"encoding/json"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/net/route"
	"golang.org/x/sys/unix"
	"rocketscience/internal/tunhelper"
)

func TestRootHelperSessionDeletesRouteWhileClientHoldsFD(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("re-run with sudo")
	}
	bin := filepath.Join(t.TempDir(), "rs-tun-helper")
	build := exec.Command("go", "build", "-o", bin, "./cmd/tun-helper")
	build.Dir = moduleRoot(t)
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(tunhelper.ServiceSocket)
	cmd := exec.Command(bin, "serve", "0")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		_ = os.Remove(tunhelper.ServiceSocket)
	})
	dest := netip.MustParsePrefix("1.0.0.0/8")
	own := netip.MustParseAddr("198.18.0.1")
	for i := 0; i < 5; i++ {
		fd := oneSession(t)
		got, err := routeGateway(dest)
		unix.Close(fd)
		if err == nil && got == own {
			t.Fatalf("session %d: 1.0.0.0/8 still via %s while the client held the utun fd", i, got)
		}
	}
}

func oneSession(t *testing.T) int {
	t.Helper()
	var conn *net.UnixConn
	deadline := time.Now().Add(5 * time.Second)
	for {
		c, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: tunhelper.ServiceSocket, Net: "unix"})
		if err == nil {
			conn = c
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer conn.Close()
	body, _ := json.Marshal(tunhelper.Request{
		MTU:          1500,
		Inet4Address: []string{"198.18.0.1/30"},
		AutoRoute:    true,
	})
	body = append(body, '\n')
	if _, err := conn.Write(body); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4096)
	oob := make([]byte, unix.CmsgSpace(4*16))
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, oobn, _, _, err := conn.ReadMsgUnix(buf, oob)
	if err != nil {
		t.Fatal(err)
	}
	var response tunhelper.Response
	if err := json.Unmarshal(buf[:n], &response); err != nil {
		t.Fatalf("response %q: %v", buf[:n], err)
	}
	if response.Error != "" {
		t.Fatal(response.Error)
	}
	cmsgs, err := unix.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		t.Fatal(err)
	}
	var fd int
	for _, cmsg := range cmsgs {
		fds, err := unix.ParseUnixRights(&cmsg)
		if err == nil && len(fds) > 0 {
			fd = fds[0]
		}
	}
	if fd == 0 {
		t.Fatal("helper sent no utun descriptor")
	}
	conn.Close()
	time.Sleep(200 * time.Millisecond)
	return fd
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(filepath.Dir(dir))
}

func routeGateway(destination netip.Prefix) (netip.Addr, error) {
	fd, err := unix.Socket(unix.AF_ROUTE, unix.SOCK_RAW, 0)
	if err != nil {
		return netip.Addr{}, err
	}
	defer unix.Close(fd)
	msg := &route.RouteMessage{
		Type:    unix.RTM_GET,
		Flags:   unix.RTF_UP | unix.RTF_STATIC | unix.RTF_GATEWAY,
		Version: unix.RTM_VERSION,
		Seq:     1,
		ID:      uintptr(os.Getpid()),
		Addrs: []route.Addr{
			unix.RTAX_DST:     &route.Inet4Addr{IP: destination.Addr().As4()},
			unix.RTAX_NETMASK: &route.Inet4Addr{IP: netip.MustParseAddr("255.0.0.0").As4()},
		},
	}
	raw, err := msg.Marshal()
	if err != nil {
		return netip.Addr{}, err
	}
	if _, err = unix.Write(fd, raw); err != nil {
		return netip.Addr{}, err
	}
	buf := make([]byte, 2048)
	n, err := unix.Read(fd, buf)
	if err != nil {
		return netip.Addr{}, err
	}
	msgs, err := route.ParseRIB(route.RIBTypeRoute, buf[:n])
	if err != nil {
		return netip.Addr{}, err
	}
	for _, item := range msgs {
		rm, ok := item.(*route.RouteMessage)
		if !ok || len(rm.Addrs) <= unix.RTAX_GATEWAY {
			continue
		}
		if addr, ok := rm.Addrs[unix.RTAX_GATEWAY].(*route.Inet4Addr); ok {
			return netip.AddrFrom4(addr.IP), nil
		}
	}
	return netip.Addr{}, os.ErrNotExist
}
