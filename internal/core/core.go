package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/metacubex/mihomo/listener"
	"gopkg.in/yaml.v3"
	"rocketscience/internal/systemproxy"
	"rocketscience/internal/tunhelper"
)

// Service owns the mihomo process and coordinates network takeover.
type Service struct {
	// savedSelections remembers Selector choices across stop/start; the core's temp work dir drops cache.db.
	savedSelections map[string]string
	// selectionStore persists savedSelections under selectionProfile (see bindSelections).
	selectionStore    *selectionStore
	selectionProfile  string
	startCancelMu     sync.Mutex
	startCancel       context.CancelFunc
	startupStatus     atomic.Pointer[Status]
	opMu              sync.Mutex
	mu                sync.Mutex
	cmd               *exec.Cmd
	done              chan struct{}
	port              int
	controllerAddress string
	modeRevision      uint64
	mixedPort         int
	secret            string
	lastError         string
	startedAt         time.Time
	updateGeo         func() error
	streamMu          sync.Mutex
	streamGeneration  uint64
	streamCancel      context.CancelFunc
	streamFinished    <-chan struct{}
	telemetry         Telemetry
	memory            MemoryUsage
	trafficSamples    [24]trafficSample
	trafficNext       int
	trafficCount      int
	logs              []LogEntry
	proxy             proxyController
	useSystemProxy    bool
	proxyBypass       *[]string
	lan               LANSettings
	useTun            bool
	tunProcess        *tunLauncher
	currentContent    string
	coreOptions       CoreOptions
	geo               geoCache
	geoWarning        string
	geoDirtyFailed    bool
	geoSession        geoSession
	runtimeDir        string
	stopping          bool
	startActive       func() error
	validateRuntime   func(context.Context, string, string) error
	tunLaunch         func(string, string) (*tunLauncher, error)
	tunReady          func(map[string]bool) bool
	tunDevice         *tunDevice
	openTun           func(context.Context, tunhelper.Request) (*tunDevice, error)
	ownsProxy         bool
	proxyWarning      string
}
type Status struct {
	Starting        bool      `json:"starting"`
	StartupStage    string    `json:"startupStage"`
	Running         bool      `json:"running"`
	Version         string    `json:"version"`
	MixedPort       int       `json:"mixedPort"`
	Mode            string    `json:"mode"`
	Error           string    `json:"error"`
	CorePath        string    `json:"corePath"`
	SystemProxy     bool      `json:"systemProxy"`
	Tun             bool      `json:"tun"`
	RecoveryPending bool      `json:"recoveryPending"`
	Warning         string    `json:"warning"`
	GeoWarning      string    `json:"geoWarning"`
	Controller      string    `json:"controller"`
	StartedAt       time.Time `json:"startedAt"`
}

func New() *Service { return &Service{proxy: newProxyController()} }
func corePath() string {
	return "embedded"
}
func (s *Service) Status() Status {
	if pending := s.startupStatus.Load(); pending != nil {
		return *pending
	}
	// A start can begin between the atomic read and acquiring mu. Recheck the
	// progress snapshot while contended instead of waiting behind a download.
	for !s.mu.TryLock() {
		if pending := s.startupStatus.Load(); pending != nil {
			return *pending
		}
		time.Sleep(5 * time.Millisecond)
	}
	defer s.mu.Unlock()
	st := Status{Running: s.cmd != nil, Error: s.lastError, CorePath: corePath(), SystemProxy: s.useSystemProxy, Tun: s.useTun, Warning: s.proxyWarning, GeoWarning: s.geoWarning}
	st.StartedAt = s.startedAt
	if s.cmd != nil {
		st.Controller = s.controllerAuthority()
	}
	if s.proxy != nil {
		pending, err := s.proxy.Pending()
		st.RecoveryPending = pending
		if err != nil {
			if st.Warning != "" {
				st.Warning += "；"
			}
			st.Warning += "无法读取系统代理恢复记录：" + err.Error()
		}
	}
	if s.cmd == nil {
		return st
	}
	st.Version = mihomoVersion()
	if ports := listener.GetPorts(); ports.MixedPort > 0 {
		st.MixedPort = ports.MixedPort
	} else {
		st.MixedPort = s.mixedPort
	}
	st.Mode = currentInProcessMode()
	return st
}

// runtimeConfig preserves the app-controlled listener and controller on every reload.
func runtimeConfig(content string, mixedPort, controllerPort int, secret, mode string) ([]byte, error) {
	return runtimeConfigWithLAN(content, mixedPort, controllerPort, secret, mode, LANSettings{})
}

func runtimeConfigWithLAN(content string, mixedPort, controllerPort int, secret, mode string, lan LANSettings) ([]byte, error) {
	return runtimeConfigWithTakeovers(content, mixedPort, controllerPort, secret, mode, lan, false)
}

func runtimeConfigWithTakeovers(content string, mixedPort, controllerPort int, secret, mode string, lan LANSettings, tun bool) ([]byte, error) {
	cfg, err := unmarshalConfigMap([]byte(content))
	if err != nil {
		return nil, fmt.Errorf("配置格式错误：%w", err)
	}
	if cfg == nil {
		return nil, errors.New("配置不能为空")
	}
	if err := validateProviderPaths(cfg); err != nil {
		return nil, err
	}
	cfg["mixed-port"] = mixedPort
	cfg["allow-lan"] = false
	cfg["bind-address"] = "127.0.0.1"
	if lan.Enabled {
		if lan.Address == "*" {
			cfg["allow-lan"] = true
			cfg["bind-address"] = "*"
		} else {
			listeners, ok := cfg["listeners"].([]any)
			if cfg["listeners"] != nil && !ok {
				return nil, errors.New("配置中的 listeners 必须为列表")
			}
			for _, entry := range listeners {
				if v, ok := entry.(map[string]any); ok && v["name"] == "rocketscience-lan" {
					return nil, errors.New("配置中已存在名为 rocketscience-lan 的监听器")
				}
			}
			cfg["listeners"] = append(listeners, map[string]any{"name": "rocketscience-lan", "type": "mixed", "listen": lan.Address, "port": mixedPort, "udp": true})
		}
		if auth, ok := cfg["authentication"].([]any); ok && len(auth) > 0 {
			prefixes, _ := cfg["skip-auth-prefixes"].([]any)
			cfg["skip-auth-prefixes"] = append(prefixes, "127.0.0.0/8", "::1/128")
		}
	}
	if tun {
		cfg["tun"] = map[string]any{"enable": true, "stack": "mixed", "auto-route": true, "auto-detect-interface": true, "dns-hijack": []string{"any:53", "tcp://any:53"}}
	} else {
		cfg["tun"] = map[string]any{"enable": false}
	}
	for _, key := range []string{"port", "socks-port", "redir-port", "tproxy-port"} {
		cfg[key] = 0
	}
	for _, key := range []string{"external-controller-unix", "external-controller-pipe", "external-controller-tls", "external-ui", "external-ui-name", "external-ui-url"} {
		delete(cfg, key)
	}
	cfg["external-controller"] = "127.0.0.1:" + strconv.Itoa(controllerPort)
	cfg["secret"] = secret
	if mode != "" {
		cfg["mode"] = mode
	}
	profileSection, ok := cfg["profile"].(map[string]any)
	if !ok && cfg["profile"] == nil {
		profileSection = map[string]any{}
	}
	if profileSection != nil {
		if _, exists := profileSection["store-fake-ip"]; !exists {
			profileSection["store-fake-ip"] = true
		}
		if _, exists := profileSection["store-selected"]; !exists {
			profileSection["store-selected"] = false
		}
		cfg["profile"] = profileSection
	}
	return yaml.Marshal(cfg)
}

func (s *Service) applyRuntimeConfigLocked(data []byte) error {
	configPath := ""
	if s.runtimeDir != "" {
		configPath = filepath.Join(s.runtimeDir, "config.yaml")
	}
	skipTunListener := s.useTun && (s.tunLaunch != nil || s.tunReady != nil)
	return applyMihomoEngineConfig(s.runtimeDir, configPath, data, true, false, "", skipTunListener, s.attachTunLocked(context.Background()))
}

// reloadIfRunning applies a validated profile without changing control credentials or takeover settings.
func (s *Service) reloadIfRunning(content string) (bool, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil {
		return false, nil
	}
	mode := currentInProcessMode()
	data, err := runtimeConfigWithOptions(content, s.mixedPort, s.port, s.secret, mode, s.lan, s.useTun, s.coreOptions)
	if err != nil {
		return true, err
	}
	if s.currentContent != "" {
		if previous, prevErr := runtimeConfigWithOptions(s.currentContent, s.mixedPort, s.port, s.secret, mode, s.lan, s.useTun, s.coreOptions); prevErr == nil && string(data) == string(previous) {
			s.currentContent = content
			return true, nil
		}
	}
	if err = s.markGeoAuto(data); err != nil {
		return true, err
	}
	if err = s.applyRuntimeConfigLocked(data); err != nil {
		return true, fmt.Errorf("重新加载配置失败：%w", err)
	}
	s.currentContent = content
	return true, nil
}

// Start validates a candidate and starts a local-only, unprivileged core.
func (s *Service) Start(content string, mixedPort int) error {
	return s.start(content, mixedPort, nil, LANSettings{}, CoreOptions{})
}

func (s *Service) start(content string, mixedPort int, bypass *[]string, lan LANSettings, options CoreOptions) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	err := s.startLocked(content, mixedPort, bypass, lan, options)
	if err == nil {
		s.mu.Lock()
		s.modeRevision++
		s.mu.Unlock()
	}
	return err
}

// startLocked serializes lifecycle operations with opMu.
func (s *Service) startLocked(content string, mixedPort int, bypass *[]string, lan LANSettings, options CoreOptions, selections ...map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil {
		return errors.New("内核已经启动")
	}
	s.publishStartup("准备配置", mixedPort)
	defer s.startupStatus.Store(nil)
	startCtx, cancelStartCtx := context.WithCancel(context.Background())
	s.startCancelMu.Lock()
	s.startCancel = cancelStartCtx
	s.startCancelMu.Unlock()
	defer func() {
		s.startCancelMu.Lock()
		s.startCancel = nil
		s.startCancelMu.Unlock()
		cancelStartCtx()
	}()
	if s.done != nil {
		select {
		case <-s.done:
		default:
			previousDone := s.done
			s.mu.Unlock()
			select {
			case <-previousDone:
			case <-time.After(10 * time.Second):
				s.mu.Lock()
				return errors.New("上一次内核清理尚未完成")
			}
			s.mu.Lock()
		}
	}
	if !s.useSystemProxy && !s.useTun {
		return errors.New("请先开启系统代理或 TUN 增强模式")
	}
	if s.proxy != nil {
		pending, err := s.proxy.Pending()
		if err != nil {
			return err
		}
		if pending {
			return errors.New("存在未恢复的系统代理，请先恢复网络")
		}
	}
	if mixedPort < 1 || mixedPort > 65535 {
		return errors.New("端口必须在 1 到 65535 之间")
	}
	if options.Inbound != nil {
		if err := validateInboundOptions(*options.Inbound, mixedPort); err != nil {
			return err
		}
	}
	if s.useTun {
		options = enabledTunDNS(options)
	}
	if err := validateCoreOptions(options); err != nil {
		return err
	}
	controller := ControllerOptions{Address: "127.0.0.1"}
	if options.Controller != nil {
		controller = *options.Controller
	}
	if err := validateControllerPorts(controller, mixedPort, options.Inbound); err != nil {
		return err
	}
	if err := validateLAN(lan); err != nil {
		return err
	}
	listenHost := "127.0.0.1"
	if lan.Enabled && lan.Address == "*" {
		listenHost = "0.0.0.0"
	}
	probe, err := net.Listen("tcp", net.JoinHostPort(listenHost, strconv.Itoa(mixedPort)))
	if err != nil {
		return portUnavailableError(mixedPort, err)
	}
	probe.Close()
	if options.Inbound != nil {
		for _, port := range []int{options.Inbound.HTTPPort, options.Inbound.SOCKSPort} {
			if port == 0 {
				continue
			}
			listener, probeErr := net.Listen("tcp", net.JoinHostPort(listenHost, strconv.Itoa(port)))
			if probeErr != nil {
				return portUnavailableError(port, probeErr)
			}
			_ = listener.Close()
		}
	}
	if lan.Enabled && lan.Address != "*" {
		probe, err = net.Listen("tcp", net.JoinHostPort(lan.Address, strconv.Itoa(mixedPort)))
		if err != nil {
			return fmt.Errorf("局域网地址或端口不可用：%w", err)
		}
		probe.Close()
	}
	bin := corePath()
	for attempt := 0; ; attempt++ {
		if attempt >= 32 {
			return errors.New("无法选择可用的控制接口端口")
		}
		ln, listenErr := net.Listen("tcp", net.JoinHostPort(controllerBind(controller), strconv.Itoa(controller.Port)))
		if listenErr != nil {
			return fmt.Errorf("控制接口地址或端口不可用：%w", listenErr)
		}
		candidate := ln.Addr().(*net.TCPAddr).Port
		_ = ln.Close()
		if candidate == mixedPort || (options.Inbound != nil && (candidate == options.Inbound.HTTPPort || candidate == options.Inbound.SOCKSPort)) {
			continue
		}
		s.port = candidate
		break
	}
	dialHost := controllerBind(controller)
	if dialHost == "0.0.0.0" {
		dialHost = "127.0.0.1"
	}
	if dialHost == "::" {
		dialHost = "::1"
	}
	s.controllerAddress = net.JoinHostPort(dialHost, strconv.Itoa(s.port))
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return err
	}
	s.secret = hex.EncodeToString(secret)
	if controller.Secret != "" {
		s.secret = controller.Secret
	}
	data, err := runtimeConfigWithOptions(content, mixedPort, s.port, s.secret, "", lan, s.useTun, options)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "rocketscience-core-")
	if err != nil {
		return err
	}
	if err = os.Chmod(dir, 0700); err != nil {
		os.RemoveAll(dir)
		return err
	}
	if warnings, recoveryErr := s.geo.recover(""); recoveryErr != nil {
		s.geoWarning = "无法检查 GEO 恢复记录：" + recoveryErr.Error()
	} else if len(warnings) > 0 {
		s.geoWarning = strings.Join(warnings, "；")
	}
	if err = s.geo.seed(dir); err != nil {
		os.RemoveAll(dir)
		return fmt.Errorf("无法准备 GEO 数据：%w", err)
	}
	configPath := filepath.Join(dir, "config.yaml")
	if err = os.WriteFile(configPath, data, 0600); err != nil {
		os.RemoveAll(dir)
		return err
	}
	validationContext, cancelValidation := context.WithTimeout(startCtx, 2*time.Minute)
	s.publishStartup("校验配置与外部资源", mixedPort)
	validateFn := s.validateRuntime
	if validateFn == nil {
		validateFn = func(ctx context.Context, d, p string) error {
			return validateRuntimeWithContext(ctx, bin, d, p)
		}
	}
	s.mu.Unlock()
	err = validateFn(validationContext, dir, configPath)
	s.mu.Lock()
	cancelValidation()
	if err != nil {
		os.RemoveAll(dir)
		if startCtx.Err() != nil {
			return errors.New("内核启动已取消")
		}
		return err
	}
	var geoCfg struct {
		AutoUpdate bool `yaml:"geo-auto-update"`
	}
	if err = yaml.Unmarshal(data, &geoCfg); err != nil {
		os.RemoveAll(dir)
		return err
	}
	session := geoSession{Version: 1, Runtime: dir, AutoUpdate: geoCfg.AutoUpdate, Dirty: true, PID: os.Getpid()}
	if err = s.geo.writeSession(session); err != nil {
		os.RemoveAll(dir)
		return fmt.Errorf("无法创建 GEO 恢复记录：%w", err)
	}
	isTun := s.useTun
	beforeTun := activeTunInterfaces()
	if isTun && s.tunLaunch != nil {
		s.publishStartup("等待虚拟网卡授权", mixedPort)
		privileged, launchErr := s.tunLaunch(bin, dir)
		if launchErr != nil {
			_ = s.geo.removeSession(dir)
			os.RemoveAll(dir)
			return launchErr
		}
		if privileged != nil {
			stopHelper := func() {
				_ = privileged.stop()
				if privileged.cmd != nil {
					_ = privileged.cmd.Wait()
				}
				privileged.finished()
			}
			select {
			case <-privileged.exited:
				waitErr := errors.New("exit status 1")
				if privileged.cmd != nil {
					if wErr := privileged.cmd.Wait(); wErr != nil {
						waitErr = wErr
					}
				}
				privileged.finished()
				_ = s.geo.removeSession(dir)
				if summary := coreLogSummary(filepath.Join(dir, "tun-core.log")); summary != "" {
					os.RemoveAll(dir)
					return fmt.Errorf("内核启动失败：%s（%v）", summary, waitErr)
				}
				os.RemoveAll(dir)
				return fmt.Errorf("内核启动失败或授权已取消：%v", waitErr)
			default:
			}
			if s.tunReady != nil && !s.tunReady(beforeTun) {
				s.mu.Unlock()
				<-startCtx.Done()
				s.mu.Lock()
				stopHelper()
				_ = s.geo.removeSession(dir)
				os.RemoveAll(dir)
				return errors.New("内核启动已取消")
			}
			stopHelper()
		}
	}
	s.publishStartup("启动 mihomo", mixedPort)
	skipTunListener := isTun && (s.tunLaunch != nil || s.tunReady != nil)
	if isTun && !skipTunListener {
		s.publishStartup("等待虚拟网卡授权", mixedPort)
	}
	if err = applyMihomoEngineConfig(dir, configPath, data, true, true, s.controllerAddress, skipTunListener, s.attachTunLocked(startCtx)); err != nil {
		stopMihomoEngine(s.controllerAddress)
		s.closeTunDeviceLocked()
		_ = s.geo.removeSession(dir)
		os.RemoveAll(dir)
		return err
	}
	if startCtx.Err() != nil {
		stopMihomoEngine(s.controllerAddress)
		s.closeTunDeviceLocked()
		_ = s.geo.removeSession(dir)
		os.RemoveAll(dir)
		return errors.New("内核启动已取消")
	}
	done := make(chan struct{})
	s.cmd = &exec.Cmd{}
	s.tunProcess = nil
	s.done = done
	s.startedAt = time.Now().UTC()
	s.mixedPort = mixedPort
	s.proxyBypass = cloneBypass(bypass)
	s.lan = lan
	s.coreOptions = options
	s.currentContent = content
	s.runtimeDir = dir
	s.geoDirtyFailed = true
	s.geoSession = session
	s.lastError = ""
	finishFailedStart := func() error {
		stopMihomoEngine(s.controllerAddress)
		s.closeTunDeviceLocked()
		s.cmd = nil
		s.tunProcess = nil
		_ = s.geo.removeSession(dir)
		_ = os.RemoveAll(dir)
		close(done)
		return nil
	}
	s.restoreSavedSelections()
	for _, selected := range selections {
		if err := s.restoreSelections(selected); err != nil {
			return errors.Join(err, finishFailedStart())
		}
	}
	// Record what the core actually uses: a saved node that no longer
	// exists falls back to the group's default, and the record follows.
	s.snapshotSelections()
	s.startStreamsAt(s.controllerAuthority(), s.secret, done)
	if s.useSystemProxy {
		s.publishStartup("应用系统代理设置", mixedPort)
		if s.proxy == nil {
			return errors.Join(errors.New("系统代理在当前平台不可用"), finishFailedStart())
		}
		if err := s.proxy.ApplyWithBypass(mixedPort, s.proxyBypass); err != nil {
			var recoveryErr error
			if errors.Is(err, systemproxy.ErrRecoveryNeeded) {
				_, recoveryErr = s.proxy.Restore()
			}
			if recoveryErr == nil {
				if stopErr := finishFailedStart(); stopErr != nil {
					return errors.Join(err, stopErr)
				}
			} else {
				s.ownsProxy = true
				s.proxyWarning = "系统代理恢复失败，请先恢复网络：" + recoveryErr.Error()
			}
			return err
		}
		s.ownsProxy = true
		s.proxyWarning = ""
	}
	session.Dirty = false
	if journalErr := s.geo.writeSession(session); journalErr != nil {
		s.geoWarning = "无法更新 GEO 恢复记录，临时文件将在退出后保留：" + journalErr.Error()
	} else {
		s.geoSession = session
		s.geoDirtyFailed = false
	}
	return nil
}

func (s *Service) handleUnexpectedExit(cause error) {
	s.mu.Lock()
	if s.cmd == nil || s.stopping {
		s.mu.Unlock()
		return
	}
	dir := s.runtimeDir
	dirty := s.geoDirtyFailed
	done := s.done
	controllerAddr := s.controllerAddress
	s.cmd = nil
	s.tunProcess = nil
	if cause != nil {
		s.lastError = "内核已退出：" + cause.Error()
	}
	s.mu.Unlock()

	stopMihomoEngine(controllerAddr)

	s.mu.Lock()
	s.closeTunDeviceLocked()
	if dirty {
		s.geoWarning = "GEO 更新未完成，先前数据库已保留；临时文件位于 " + dir
	} else if cacheErr := s.geo.save(dir); cacheErr != nil {
		s.geoWarning = "GEO 数据未能保存，临时文件保留在 " + dir + "：" + cacheErr.Error()
	} else if journalErr := s.geo.removeSession(dir); journalErr != nil {
		s.geoWarning = "GEO 恢复记录未能清理，临时文件保留在 " + dir
	} else {
		_ = os.RemoveAll(dir)
	}
	if done != nil {
		close(done)
	}
	s.mu.Unlock()
	s.recoverAfterExit()
}

func (s *Service) Stop() error {
	s.startCancelMu.Lock()
	if s.startCancel != nil {
		s.startCancel()
	}
	s.startCancelMu.Unlock()
	s.opMu.Lock()
	defer s.opMu.Unlock()
	s.mu.Lock()
	s.modeRevision++
	s.mu.Unlock()
	return s.stopLocked()
}

// stopLocked is called with opMu held, including when the last takeover is disabled.
func (s *Service) stopLocked() error {
	s.mu.Lock()
	if s.cmd == nil {
		s.mu.Unlock()
		return nil
	}
	s.snapshotSelections()
	if s.proxy != nil && s.ownsProxy {
		conflicts, err := s.proxy.Restore()
		if err != nil {
			s.mu.Unlock()
			return err
		}
		if len(conflicts) > 0 {
			s.proxyWarning = "部分网络服务已被其他程序修改，未覆盖其设置"
		}
		s.ownsProxy = false
	}
	s.stopping = true
	dir := s.runtimeDir
	dirty := s.geoDirtyFailed
	done := s.done
	controllerAddr := s.controllerAddress
	s.mu.Unlock()

	stopMihomoEngine(controllerAddr)

	s.mu.Lock()
	s.closeTunDeviceLocked()
	s.cmd = nil
	s.tunProcess = nil
	s.stopping = false
	if dirty {
		s.geoWarning = "GEO 更新未完成，先前数据库已保留；临时文件位于 " + dir
	} else if cacheErr := s.geo.save(dir); cacheErr != nil {
		s.geoWarning = "GEO 数据未能保存，临时文件保留在 " + dir + "：" + cacheErr.Error()
	} else if journalErr := s.geo.removeSession(dir); journalErr != nil {
		s.geoWarning = "GEO 恢复记录未能清理，临时文件保留在 " + dir
	} else {
		_ = os.RemoveAll(dir)
	}
	if done != nil {
		close(done)
	}
	s.mu.Unlock()
	return nil
}
func (s *Service) Shutdown() { _ = s.Stop() }
