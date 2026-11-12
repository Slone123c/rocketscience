package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/metacubex/bbolt"
	"github.com/metacubex/mihomo/adapter/outboundgroup"
	"github.com/metacubex/mihomo/component/profile/cachefile"
	"github.com/metacubex/mihomo/component/updater"
	C "github.com/metacubex/mihomo/constant"
	"github.com/metacubex/mihomo/dns"
	"github.com/metacubex/mihomo/hub"
	"github.com/metacubex/mihomo/hub/executor"
	"github.com/metacubex/mihomo/hub/route"
	"github.com/metacubex/mihomo/listener"
	LC "github.com/metacubex/mihomo/listener/config"
	mihomoLog "github.com/metacubex/mihomo/log"
	"github.com/metacubex/mihomo/ntp/ntp"
	"github.com/metacubex/mihomo/tunnel"
	"github.com/metacubex/mihomo/tunnel/statistic"
)

var mihomoEngineMu sync.Mutex

func openMihomoCacheDB(dir string) {
	c := cachefile.Cache()
	target := filepath.Join(dir, "cache.db")
	if c.DB != nil {
		if c.DB.Path() == target {
			return
		}
		_ = c.DB.Close()
		c.DB = nil
	}
	if dir == "" {
		return
	}
	db, err := bbolt.Open(target, 0o600, &bbolt.Options{Timeout: time.Second, NoStatistics: true})
	if err == nil {
		c.DB = db
	}
}

func closeMihomoCacheDB() {
	c := cachefile.Cache()
	if c.DB != nil {
		_ = c.DB.Close()
		c.DB = nil
	}
}

func parseConfigInDir(dir, path string, content []byte) error {
	mihomoEngineMu.Lock()
	defer mihomoEngineMu.Unlock()
	prevHome := C.Path.HomeDir()
	prevConfig := C.Path.Config()
	c := cachefile.Cache()
	hadDB := c.DB != nil
	if dir != "" {
		C.SetHomeDir(dir)
	}
	if path != "" {
		C.SetConfig(path)
	}
	defer func() {
		if !hadDB && c.DB != nil {
			_ = c.DB.Close()
			c.DB = nil
		}
		C.SetHomeDir(prevHome)
		C.SetConfig(prevConfig)
	}()
	_, err := executor.ParseWithBytes(content)
	return err
}

// applyMihomoEngineConfig starts or reloads the engine. attachTun supplies the
// descriptor of a device created outside the app (see tunDevice); without it,
// or with skipTunListener, the TUN listener is left out.
func applyMihomoEngineConfig(dir, configPath string, data []byte, force bool, withController bool, controllerAddr string, skipTunListener bool, attachTun func(LC.Tun) (int, error)) error {
	mihomoEngineMu.Lock()
	defer mihomoEngineMu.Unlock()
	if dir != "" {
		C.SetHomeDir(dir)
		openMihomoCacheDB(dir)
	}
	if configPath != "" {
		C.SetConfig(configPath)
		_ = os.WriteFile(configPath, data, 0o600)
	}
	cfg, err := executor.ParseWithBytes(data)
	if err != nil {
		return fmt.Errorf("mihomo：%w", err)
	}
	wantTun := cfg.General.Tun.Enable
	if skipTunListener || attachTun == nil {
		cfg.General.Tun.Enable = false
	} else if wantTun {
		fd, err := attachTun(cfg.General.Tun)
		if err != nil {
			return err
		}
		// rs-tun-helper already added the routes and configured the control
		// socket, including recvmsgx. sing-tun must not repeat that
		// setsockopt: macOS rejects it and the listener is dropped.
		cfg.General.Tun.FileDescriptor = fd
		cfg.General.Tun.AutoRoute = false
	}
	// ApplyConfig only logs a failed TUN listener, so listen for that line.
	var tunLog <-chan mihomoLog.Event
	if wantTun && !skipTunListener && attachTun != nil {
		sub := mihomoLog.Subscribe()
		defer mihomoLog.UnSubscribe(sub)
		tunLog = sub
	}
	route.SetEmbedMode(true)
	if withController {
		hub.ApplyConfig(cfg)
		if controllerAddr != "" {
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				conn, dialErr := net.DialTimeout("tcp", controllerAddr, 25*time.Millisecond)
				if dialErr == nil {
					_ = conn.Close()
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
		}
	} else {
		executor.ApplyConfig(cfg, force)
	}
	if wantTun && !skipTunListener && attachTun != nil && !listener.GetTunConf().Enable {
		// ApplyConfig logs the listener error and continues, so the only
		// signal left here is Enable=false. The log names the real cause.
		reason := lastTunError(tunLog)
		tunDebugf("mihomo TUN 监听启动失败：%s", reason)
		return fmt.Errorf("TUN 启动失败：虚拟网卡未能就绪（%s）", reason)
	}
	if wantTun && !skipTunListener && attachTun != nil {
		conf := listener.GetTunConf()
		tunDebugf("mihomo TUN 监听已启动：fd=%d stack=%s auto-route=%v", conf.FileDescriptor, conf.Stack, conf.AutoRoute)
	}
	return nil
}

// lastTunError returns the listener failure mihomo logged while applying.
func lastTunError(events <-chan mihomoLog.Event) string {
	// Log events reach subscribers asynchronously.
	timeout := time.After(300 * time.Millisecond)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return "原因未记录，请查看日志"
			}
			if ev.LogLevel == mihomoLog.ERROR && strings.Contains(ev.Payload, "Start TUN listening error") {
				return strings.TrimSpace(strings.TrimPrefix(ev.Payload, "Start TUN listening error:"))
			}
		case <-timeout:
			return "原因未记录，请查看日志"
		}
	}
}

func stopMihomoEngine(controllerAddr string) {
	mihomoEngineMu.Lock()
	defer mihomoEngineMu.Unlock()
	tunnel.OnSuspend()
	listener.PatchInboundListeners(nil, tunnel.Tunnel, true)
	listener.ReCreateHTTP(0, tunnel.Tunnel)
	listener.ReCreateSocks(0, tunnel.Tunnel)
	listener.ReCreateRedir(0, tunnel.Tunnel)
	listener.ReCreateTProxy(0, tunnel.Tunnel)
	listener.ReCreateMixed(0, tunnel.Tunnel)
	listener.ReCreateShadowSocks("", tunnel.Tunnel)
	listener.ReCreateVmess("", tunnel.Tunnel)
	listener.ReCreateTuic(LC.TuicServer{}, tunnel.Tunnel)
	listener.PatchTunnel(nil, tunnel.Tunnel)
	// Also forgets the last TUN config: otherwise the next start with the same
	// config counts as unchanged and never opens a listener.
	listener.ReCreateTun(LC.Tun{}, tunnel.Tunnel)
	listener.Cleanup()
	dns.ReCreateServer("", nil, nil)
	ntp.ReCreateNTPService("", 0, "", nil, false)
	route.ReCreateServer(&route.Config{})
	if controllerAddr != "" {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			conn, err := net.DialTimeout("tcp", controllerAddr, 20*time.Millisecond)
			if err != nil {
				break
			}
			_ = conn.Close()
			time.Sleep(10 * time.Millisecond)
		}
	}
	statistic.DefaultManager.Range(func(c statistic.Tracker) bool {
		_ = c.Close()
		return true
	})
	executor.Shutdown()
	closeMihomoCacheDB()
}

func currentInProcessMode() string {
	return strings.ToLower(tunnel.Mode().String())
}

func setInProcessMode(mode string) error {
	var m tunnel.TunnelMode
	if err := m.UnmarshalText([]byte(mode)); err != nil {
		return err
	}
	tunnel.SetMode(m)
	return nil
}

func inProcessProxiesMap() (map[string]proxyDetail, error) {
	raw := tunnel.Proxies()
	out := make(map[string]proxyDetail, len(raw))
	for name, p := range raw {
		b, err := json.Marshal(p)
		if err != nil {
			return nil, err
		}
		var item proxyDetail
		if err := json.Unmarshal(b, &item); err != nil {
			return nil, err
		}
		if item.Name == "" {
			item.Name = name
		}
		out[name] = item
	}
	return out, nil
}

func inProcessRules() []Rule {
	rawRules := tunnel.Rules()
	rules := make([]Rule, 0, len(rawRules))
	for _, rule := range rawRules {
		rules = append(rules, Rule{
			Type:    rule.RuleType().String(),
			Payload: rule.Payload(),
			Proxy:   rule.Adapter(),
		})
	}
	return rules
}

func inProcessConnectionsSnapshot() ConnectionSnapshot {
	snap := statistic.DefaultManager.Snapshot()
	if snap == nil {
		return ConnectionSnapshot{Connections: []Connection{}}
	}
	conns := make([]Connection, 0, len(snap.Connections))
	for _, info := range snap.Connections {
		if info == nil {
			continue
		}
		var meta ConnectionMetadata
		if info.Metadata != nil {
			dstIP := ""
			if info.Metadata.DstIP.IsValid() {
				dstIP = info.Metadata.DstIP.String()
			}
			dstPort := ""
			if info.Metadata.DstPort != 0 {
				dstPort = fmt.Sprint(info.Metadata.DstPort)
			}
			meta = ConnectionMetadata{
				Host:            info.Metadata.Host,
				DestinationIP:   dstIP,
				DestinationPort: dstPort,
				Network:         info.Metadata.NetWork.String(),
				Process:         info.Metadata.Process,
				ProcessPath:     info.Metadata.ProcessPath,
			}
		}
		chains := make([]string, len(info.Chain))
		copy(chains, info.Chain)
		conns = append(conns, Connection{
			ID:          info.UUID.String(),
			Metadata:    meta,
			Upload:      info.UploadTotal.Load(),
			Download:    info.DownloadTotal.Load(),
			Start:       info.Start.Format(time.RFC3339Nano),
			Chains:      chains,
			Rule:        info.Rule,
			RulePayload: info.RulePayload,
		})
	}
	return ConnectionSnapshot{
		Connections:   conns,
		UploadTotal:   snap.UploadTotal,
		DownloadTotal: snap.DownloadTotal,
	}
}

func testInProcessGroupDelay(ctx context.Context, groupName, target string, timeoutMS int) (map[string]int, error) {
	proxy, ok := tunnel.Proxies()[groupName]
	if !ok {
		return nil, fmt.Errorf("测速失败：未找到策略组 %s", groupName)
	}
	group, ok := proxy.Adapter().(outboundgroup.ProxyGroup)
	if !ok {
		return nil, fmt.Errorf("测速失败：%s 不是策略组", groupName)
	}
	if selectAble, ok := proxy.Adapter().(outboundgroup.SelectAble); ok && proxy.Type() != C.Selector {
		selectAble.ForceSet("")
		cachefile.Cache().SetSelected(proxy.Name(), "")
	}
	testCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()
	dm, err := group.URLTest(testCtx, target, nil)
	if err != nil {
		return nil, fmt.Errorf("测速失败：%w", err)
	}
	result := make(map[string]int, len(dm))
	for k, v := range dm {
		result[k] = int(v)
	}
	return result, nil
}

func selectInProcessProxy(group, node string) error {
	proxies := tunnel.Proxies()
	proxy, exist := proxies[group]
	if !exist {
		return fmt.Errorf("策略组 %s 不存在", group)
	}
	selector, ok := proxy.Adapter().(outboundgroup.SelectAble)
	if !ok {
		return errors.New("只有手动选择的策略组可以指定节点")
	}
	if err := selector.Set(node); err != nil {
		return err
	}
	cachefile.Cache().SetSelected(proxy.Name(), node)
	return nil
}

func closeAllInProcessConnections() {
	statistic.DefaultManager.Range(func(c statistic.Tracker) bool {
		_ = c.Close()
		return true
	})
}

func closeInProcessConnection(id string) error {
	if c := statistic.DefaultManager.Get(id); c != nil {
		_ = c.Close()
	}
	return nil
}

func updateInProcessGeoDatabases() error {
	return updater.UpdateGeoDatabases()
}

func closeGroupInProcessConnections(group string) {
	if group == "" {
		return
	}
	statistic.DefaultManager.Range(func(c statistic.Tracker) bool {
		info := c.Info()
		if info == nil {
			return true
		}
		for _, hop := range info.Chain {
			if hop == group {
				_ = c.Close()
				break
			}
		}
		return true
	})
}

func validateRuntimeWithContext(ctx context.Context, _, dir, path string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("mihomo 校验未完成，请检查配置中的外部资源和网络连接：%w", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("mihomo 校验未完成，请检查配置中的外部资源和网络连接：%w", err)
		}
		return fmt.Errorf("mihomo 校验失败：%v", err)
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- parseConfigInDir(dir, path, content)
	}()
	select {
	case <-ctx.Done():
		return fmt.Errorf("mihomo 校验未完成，请检查配置中的外部资源和网络连接：%w", ctx.Err())
	case err := <-errCh:
		if ctx.Err() != nil {
			return fmt.Errorf("mihomo 校验未完成，请检查配置中的外部资源和网络连接：%w", err)
		}
		if err != nil {
			return fmt.Errorf("mihomo 校验失败：%v", err)
		}
		return nil
	}
}
