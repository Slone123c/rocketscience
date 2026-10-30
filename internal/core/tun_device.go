package core

import (
	"context"
	"errors"
	"net/netip"

	"github.com/metacubex/mihomo/listener"
	LC "github.com/metacubex/mihomo/listener/config"
	"rocketscience/internal/tunhelper"
)

// tunRequest asks rs-tun-helper for the device mihomo's parsed TUN section
// describes. mihomo then attaches to it by descriptor without auto-route, since
// the helper already added the routes.
func tunRequest(conf LC.Tun) tunhelper.Request {
	mtu := conf.MTU
	if mtu == 0 {
		mtu = 9000 // mihomo's default
	}
	routes := append(append(append([]netip.Prefix{}, conf.RouteAddress...), conf.Inet4RouteAddress...), conf.Inet6RouteAddress...)
	excludes := append(append(append([]netip.Prefix{}, conf.RouteExcludeAddress...), conf.Inet4RouteExcludeAddress...), conf.Inet6RouteExcludeAddress...)
	return tunhelper.Request{
		Name:         conf.Device,
		MTU:          mtu,
		Inet4Address: tunhelper.PrefixStrings(conf.Inet4Address),
		Inet6Address: tunhelper.PrefixStrings(conf.Inet6Address),
		AutoRoute:    conf.AutoRoute,
		DNSMode:      conf.DNSMode,
		RecvMsgX:     conf.RecvMsgX,
		RouteAddress: tunhelper.PrefixStrings(routes),
		RouteExclude: tunhelper.PrefixStrings(excludes),
	}
}

// attachTunLocked gives mihomo a descriptor for the TUN device, asking for
// administrator privileges the first time in a session. Called with mu held.
func (s *Service) attachTunLocked(ctx context.Context) func(LC.Tun) (int, error) {
	return func(conf LC.Tun) (int, error) {
		if s.tunDevice == nil {
			open := s.openTun
			if open == nil {
				open = openTunDevice
			}
			device, err := open(ctx, tunRequest(conf))
			if err != nil {
				return -1, err
			}
			s.tunDevice = device
		} else if current := listener.GetTunConf(); current.Enable && current.FileDescriptor > 0 {
			// A reload with the same settings keeps mihomo's listener, so hand
			// back the descriptor it already owns instead of a new one.
			next := conf
			next.FileDescriptor, next.AutoRoute = current.FileDescriptor, false
			next.Sort()
			if next.Equal(current) {
				return current.FileDescriptor, nil
			}
		}
		return s.tunDevice.dup()
	}
}

// closeTunDeviceLocked releases the device after mihomo's listener stopped.
// Called with mu held.
func (s *Service) closeTunDeviceLocked() {
	if s.tunDevice != nil {
		s.tunDevice.Close()
		s.tunDevice = nil
	}
}

// TunServiceInstalled reports whether the TUN service is installed, so turning
// TUN on needs no password.
func (s *Service) TunServiceInstalled() bool { return tunServiceInstalled() }

// UninstallTunService removes the TUN service after an administrator prompt.
// The next TUN start installs it again.
func (s *Service) UninstallTunService() error {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	s.mu.Lock()
	busy := s.cmd != nil && s.useTun
	s.mu.Unlock()
	if busy {
		return errors.New("TUN 正在使用，请先关闭 TUN 再卸载服务")
	}
	if !tunServiceInstalled() {
		return nil
	}
	return uninstallTunService()
}
