package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"rocketscience/internal/systemproxy"
)

// Service owns the mihomo process and coordinates network takeover.
type Service struct {
	// savedSelections remembers Selector choices across stop/start; the core's temp work dir drops cache.db.
	savedSelections   map[string]string
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
	tunLaunch         func(string, string) (*tunLauncher, error)
	tunReady          func(map[string]bool) bool
	ownsProxy         bool
	proxyWarning      string
}
type Status struct {
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
	if p := os.Getenv("ROCKETSCIENCE_MIHOMO"); p != "" {
		return p
	}
	if executable, err := os.Executable(); err == nil {
		path := filepath.Join(filepath.Dir(executable), "..", "Resources", "bin", "mihomo")
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path
		}
	}
	if info, err := os.Stat("resources/bin/mihomo"); err == nil && info.Mode().IsRegular() {
		return "resources/bin/mihomo"
	}
	if p, err := exec.LookPath("mihomo"); err == nil {
		return p
	}
	return ""
}
func (s *Service) Status() Status {
	s.mu.Lock()
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
	var v struct {
		Version string `json:"version"`
	}
	if err := s.get("/version", &v); err != nil {
		st.Error = err.Error()
		return st
	}
	st.Version = v.Version
	var c struct {
		Port int    `json:"mixed-port"`
		Mode string `json:"mode"`
	}
	if err := s.get("/configs", &c); err != nil {
		st.Error = err.Error()
		return st
	}
	st.MixedPort = c.Port
	st.Mode = c.Mode
	return st
}
func (s *Service) get(path string, out any) error {
	req, err := http.NewRequest("GET", "http://"+s.controllerAuthority()+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.secret)
	client := http.Client{Timeout: time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("内核接口返回 %s", res.Status)
	}
	return json.NewDecoder(res.Body).Decode(out)
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
	for _, key := range []string{"external-controller-unix", "external-controller-pipe", "external-controller-tls"} {
		delete(cfg, key)
	}
	cfg["external-controller"] = "127.0.0.1:" + strconv.Itoa(controllerPort)
	cfg["secret"] = secret
	if mode != "" {
		cfg["mode"] = mode
	}
	return yaml.Marshal(cfg)
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
	var current struct {
		Mode string `json:"mode"`
	}
	if err := s.get("/configs", &current); err != nil {
		return true, err
	}
	data, err := runtimeConfigWithOptions(content, s.mixedPort, s.port, s.secret, current.Mode, s.lan, s.useTun, s.coreOptions)
	if err != nil {
		return true, err
	}
	if err = s.markGeoAuto(data); err != nil {
		return true, err
	}
	if err = s.request("PUT", "/configs?force=true", map[string]string{"payload": string(data)}); err != nil {
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
	if bin == "" {
		return errors.New("未找到 mihomo。请按项目说明安装内核")
	}
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
	validationContext, cancelValidation := context.WithTimeout(context.Background(), 2*time.Minute)
	err = validateRuntimeWithContext(validationContext, bin, dir, configPath)
	cancelValidation()
	if err != nil {
		os.RemoveAll(dir)
		return err
	}
	var geoCfg struct {
		AutoUpdate bool `yaml:"geo-auto-update"`
	}
	if err = yaml.Unmarshal(data, &geoCfg); err != nil {
		os.RemoveAll(dir)
		return err
	}
	session := geoSession{Version: 1, Runtime: dir, AutoUpdate: geoCfg.AutoUpdate, Dirty: true}
	if err = s.geo.writeSession(session); err != nil {
		os.RemoveAll(dir)
		return fmt.Errorf("无法创建 GEO 恢复记录：%w", err)
	}
	var cmd *exec.Cmd
	var privileged *tunLauncher
	isTun := s.useTun
	beforeTun := activeTunInterfaces()
	if isTun {
		launch := s.tunLaunch
		if launch == nil {
			launch = launchTun
		}
		privileged, err = launch(bin, dir)
		if err != nil {
			_ = s.geo.removeSession(dir)
			os.RemoveAll(dir)
			return err
		}
		cmd = privileged.cmd
	} else {
		privileged, err = launchUnprivileged(bin, dir)
		if err != nil {
			_ = s.geo.removeSession(dir)
			os.RemoveAll(dir)
			return err
		}
		cmd = privileged.cmd
	}
	session.PID = cmd.Process.Pid
	if err = s.geo.writeSession(session); err != nil {
		if privileged != nil {
			_ = privileged.stop()
		} else {
			_ = cmd.Process.Kill()
		}
		go func() {
			_ = cmd.Wait()
			if privileged != nil {
				privileged.finished()
			}
		}()
		return fmt.Errorf("无法记录内核状态，临时文件保留：%w", err)
	}
	done := make(chan struct{})
	exited := make(chan struct{})
	var waitErr error
	s.cmd = cmd
	s.tunProcess = privileged
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
	go func() {
		waitErr = cmd.Wait()
		if privileged != nil {
			privileged.finished()
		}
		var orphanErr error
		if !isTun && privileged != nil {
			orphanErr = cleanupOrdinaryOrphan(dir, bin)
		}
		close(exited)
		s.mu.Lock()
		unexpected := s.cmd == cmd && !s.stopping
		dirty := s.geoDirtyFailed
		if orphanErr != nil {
			dirty = true
			s.proxyWarning = "无法确认内核已退出，请检查并恢复网络：" + orphanErr.Error()
		}
		if s.cmd == cmd {
			s.cmd = nil
			s.tunProcess = nil
			if unexpected && waitErr != nil {
				s.lastError = "内核已退出：" + waitErr.Error()
			}
		}
		s.mu.Unlock()
		if dirty {
			s.mu.Lock()
			s.geoWarning = "GEO 更新未完成，先前数据库已保留；临时文件位于 " + dir
			s.mu.Unlock()
		} else if cacheErr := s.geo.save(dir); cacheErr != nil {
			s.mu.Lock()
			s.geoWarning = "GEO 数据未能保存，临时文件保留在 " + dir + "：" + cacheErr.Error()
			s.mu.Unlock()
		} else if journalErr := s.geo.removeSession(dir); journalErr != nil {
			s.mu.Lock()
			s.geoWarning = "GEO 恢复记录未能清理，临时文件保留在 " + dir
			s.mu.Unlock()
		} else {
			_ = os.RemoveAll(dir)
		}
		close(done)
		if unexpected && orphanErr == nil {
			s.recoverAfterExit()
		}
	}()
	cancelStart := func() {
		if privileged != nil {
			_ = privileged.stop()
		} else {
			_ = cmd.Process.Kill()
		}
	}
	finishFailedStart := func() error {
		cancelStart()
		select {
		case <-exited:
			s.cmd = nil
			s.tunProcess = nil
			return nil
		case <-time.After(6 * time.Second):
			return errors.New("内核停止尚未完成，请重试停止并恢复网络")
		}
	}
	timeout := 8 * time.Second
	if isTun {
		timeout = 2 * time.Minute
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-exited:
			s.cmd = nil
			s.tunProcess = nil
			return fmt.Errorf("内核启动失败或授权已取消：%v", waitErr)
		default:
		}
		if isTun {
			if failure := tunLogFailure(filepath.Join(dir, "tun-core.log")); failure != "" {
				return errors.Join(errors.New("TUN 启动失败："+failure), finishFailedStart())
			}
		}
		var v any
		tunReady := !isTun
		ready := s.tunReady
		if ready == nil {
			ready = hasNewTunInterface
		}
		if isTun && ready(beforeTun) {
			var state struct {
				Tun struct {
					Enable bool `json:"enable"`
				} `json:"tun"`
			}
			tunReady = s.get("/configs", &state) == nil && state.Tun.Enable
		}
		if s.get("/version", &v) == nil && tunReady {
			s.restoreSavedSelections()
			for _, selected := range selections {
				if err := s.restoreSelections(selected); err != nil {
					return errors.Join(err, finishFailedStart())
				}
			}
			s.startStreamsAt(s.controllerAuthority(), s.secret, done)
			if s.useSystemProxy {
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
		time.Sleep(100 * time.Millisecond)
	}
	return errors.Join(errors.New("内核未在规定时间内就绪，请检查授权、端口及配置"), finishFailedStart())
}
func (s *Service) Stop() error {
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
	cmd := s.cmd
	done := s.done
	tunProcess := s.tunProcess
	if cmd == nil {
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
	s.mu.Unlock()
	var err error
	if tunProcess != nil {
		err = tunProcess.stop()
	} else {
		err = cmd.Process.Signal(os.Interrupt)
		if err != nil {
			_ = cmd.Process.Kill()
		}
	}
	wait := 3 * time.Second
	if tunProcess != nil {
		wait = 12 * time.Second
	}
	select {
	case <-done:
	case <-time.After(wait):
		if tunProcess == nil {
			_ = cmd.Process.Kill()
			<-done
		} else {
			s.mu.Lock()
			s.stopping = false
			s.mu.Unlock()
			return errors.Join(errors.New("TUN 内核尚未停止，请重试恢复网络"), err)
		}
	}
	s.mu.Lock()
	s.stopping = false
	s.mu.Unlock()
	return nil
}
func (s *Service) Shutdown() { _ = s.Stop() }
