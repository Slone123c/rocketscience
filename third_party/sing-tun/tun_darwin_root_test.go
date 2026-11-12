//go:build darwin && roottest

package tun

import (
	"errors"
	"net/netip"
	"testing"

	"golang.org/x/sys/unix"
)

func TestRootCloseDeletesAutoRouteWhileDupHeld(t *testing.T) {
	device := openAutoRoute(t, "198.18.0.1/30", netip.Addr{})
	native := device.(*NativeTun)
	dup, err := unix.Dup(native.tunFd)
	if err != nil {
		t.Fatal(err)
	}
	dest := netip.MustParsePrefix("1.0.0.0/8")
	own := netip.MustParseAddr("198.18.0.1")
	if got := mustGateway(t, dest); got != own {
		t.Fatalf("gateway %s, want %s", got, own)
	}
	if err := device.Close(); err != nil {
		t.Fatal(err)
	}
	if got, err := gatewayOf(dest); err == nil && got == own {
		t.Fatalf("1.0.0.0/8 still via %s after Close, with a dup fd open", got)
	}
	unix.Close(dup)
}

func TestRootSecondDeviceReplacesOwnLeftover(t *testing.T) {
	first := openAutoRoute(t, "198.18.0.1/30", netip.Addr{})
	dest := netip.MustParsePrefix("1.0.0.0/8")
	own := netip.MustParseAddr("198.18.0.1")
	second := openAutoRoute(t, "198.18.0.5/30", own)
	if got := mustGateway(t, dest); got != own {
		t.Fatalf("gateway %s, want %s", got, own)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if got, err := gatewayOf(dest); err == nil && got == own {
		t.Fatalf("own route %s survived both Close calls", got)
	}
}

func TestRootForeignGatewayIsLeftAlone(t *testing.T) {
	foreignDevice := openAutoRoute(t, "203.0.113.1/30", netip.Addr{})
	defer foreignDevice.Close()
	_, err := New(Options{
		Name:         freeUtun(t),
		MTU:          1500,
		Inet4Address: []netip.Prefix{netip.MustParsePrefix("198.18.0.1/30")},
		Inet4Gateway: netip.MustParseAddr("198.18.0.1"),
		AutoRoute:    true,
	})
	if !errors.Is(err, unix.EEXIST) {
		if err == nil {
			t.Fatal("New replaced a foreign 1.0.0.0/8 route")
		}
		t.Fatalf("New err = %v, want EEXIST", err)
	}
	if got := mustGateway(t, netip.MustParsePrefix("1.0.0.0/8")); got != netip.MustParseAddr("203.0.113.1") {
		t.Fatalf("foreign gateway changed to %s", got)
	}
}

func openAutoRoute(t *testing.T, address string, gateway netip.Addr) Tun {
	t.Helper()
	device, err := New(Options{
		Name:         freeUtun(t),
		MTU:          1500,
		Inet4Address: []netip.Prefix{netip.MustParsePrefix(address)},
		Inet4Gateway: gateway,
		AutoRoute:    true,
	})
	if err != nil {
		t.Fatalf("New %s: %v", address, err)
	}
	return device
}

func freeUtun(t *testing.T) string {
	t.Helper()
	fd := mustSocket(t)
	for i := 10; i < 40; i++ {
		name := "utun" + itoa(i)
		if _, err := unix.IoctlGetIfreqMTU(fd, name); err != nil {
			return name
		}
	}
	t.Fatal("no free utun")
	return ""
}

func mustSocket(t *testing.T) int {
	t.Helper()
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { unix.Close(fd) })
	return fd
}

func mustGateway(t *testing.T, dest netip.Prefix) netip.Addr {
	t.Helper()
	got, err := gatewayOf(dest)
	if err != nil {
		t.Fatalf("gateway %s: %v", dest, err)
	}
	return got
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [4]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
