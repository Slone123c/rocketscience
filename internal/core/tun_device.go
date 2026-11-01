package core

import (
	"context"
	"errors"
	"net/netip"
	"runtime"
	"strings"

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
			request := tunRequest(conf)
			tunDebugf("请求虚拟网卡：name=%q mtu=%d inet4=%v inet6=%v auto-route=%v dns-mode=%q 路由 %d 条 排除 %d 条；stack=%s dns-hijack=%v auto-detect-interface=%v",
				request.Name, request.MTU, request.Inet4Address, request.Inet6Address, request.AutoRoute, request.DNSMode, len(request.RouteAddress), len(request.RouteExclude), conf.Stack, conf.DNSHijack, conf.AutoDetectInterface)
			device, err := open(ctx, request)
			if err != nil {
				tunDebugf("获取虚拟网卡失败：%v", err)
				return -1, err
			}
			tunDebugf("已获得虚拟网卡 %s", device.name)
			s.tunDevice = device
		} else if current := listener.GetTunConf(); current.Enable && current.FileDescriptor > 0 {
			// A reload with the same settings keeps mihomo's listener, so hand
			// back the descriptor it already owns instead of a new one.
			next := conf
			next.FileDescriptor, next.AutoRoute = current.FileDescriptor, false
			next.Sort()
			if next.Equal(current) {
				tunDebugf("重新加载：TUN 参数未变，沿用描述符 %d", current.FileDescriptor)
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
		tunDebugf("释放虚拟网卡 %s（调用来源：%s）", s.tunDevice.name, callers())
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

// callers names the functions that led here, so the debug log shows why a
// device was released (a stop, a TUN switch, a failed start).
func callers() string {
	pc := make([]uintptr, 8)
	frames := runtime.CallersFrames(pc[:runtime.Callers(3, pc)])
	var names []string
	for {
		frame, more := frames.Next()
		name := frame.Function[strings.LastIndex(frame.Function, "/")+1:]
		if strings.HasPrefix(name, "core.") {
			names = append(names, strings.TrimPrefix(name, "core."))
		}
		if !more {
			break
		}
	}
	return strings.Join(names, " ← ")
}
