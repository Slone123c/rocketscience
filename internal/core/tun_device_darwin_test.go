package core

import (
	"context"
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	LC "github.com/metacubex/mihomo/listener/config"
	"rocketscience/internal/tunhelper"
)

func TestTunRequestFollowsParsedConfig(t *testing.T) {
	request := tunRequest(LC.Tun{
		Inet4Address:        []netip.Prefix{netip.MustParsePrefix("198.18.0.1/30")},
		AutoRoute:           true,
		RecvMsgX:            true,
		RouteAddress:        []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
		Inet6RouteAddress:   []netip.Prefix{netip.MustParsePrefix("fd00::/8")},
		RouteExcludeAddress: []netip.Prefix{netip.MustParsePrefix("192.168.0.0/16")},
	})
	if request.MTU != 9000 || !request.AutoRoute || !request.RecvMsgX || request.Inet4Address[0] != "198.18.0.1/30" {
		t.Fatalf("request = %+v", request)
	}
	if len(request.RouteAddress) != 2 || request.RouteExclude[0] != "192.168.0.0/16" {
		t.Fatalf("routes = %v, excludes = %v", request.RouteAddress, request.RouteExclude)
	}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
}

// The device is opened once per session; mihomo gets duplicates it may close.
func TestAttachTunOpensOnceAndHandsOutDuplicates(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_TUN_LOG", filepath.Join(t.TempDir(), "tun.log"))
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	opened := 0
	s := &Service{openTun: func(_ context.Context, request tunhelper.Request) (*tunDevice, error) {
		opened++
		if request.Inet4Address[0] != "198.18.0.1/30" {
			t.Fatalf("request = %+v", request)
		}
		return &tunDevice{file: reader, name: "utun9"}, nil
	}}
	attach := s.attachTunLocked(context.Background())
	conf := LC.Tun{Enable: true, Inet4Address: []netip.Prefix{netip.MustParsePrefix("198.18.0.1/30")}}
	first, err := attach(conf)
	if err != nil {
		t.Fatal(err)
	}
	second, err := attach(conf)
	if err != nil {
		t.Fatal(err)
	}
	if opened != 1 || first == second || first == int(reader.Fd()) {
		t.Fatalf("opened %d times, descriptors %d %d (original %d)", opened, first, second, reader.Fd())
	}
	os.NewFile(uintptr(first), "").Close()
	os.NewFile(uintptr(second), "").Close()
	s.closeTunDeviceLocked()
	if s.tunDevice != nil {
		t.Fatal("device kept after close")
	}
	if _, err := reader.Stat(); err == nil {
		t.Fatal("original descriptor still open")
	}
}
