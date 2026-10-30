package core

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	"gopkg.in/yaml.v3"
)

// A nil field means the active profile controls that setting.
type CoreOptions struct {
	LogLevel          *string            `json:"logLevel"`
	IPv6              *bool              `json:"ipv6"`
	TCPConcurrent     *bool              `json:"tcpConcurrent"`
	UnifiedDelay      *bool              `json:"unifiedDelay"`
	FindProcessMode   *string            `json:"findProcessMode"`
	KeepAliveInterval *int               `json:"keepAliveInterval"`
	Sniffer           *bool              `json:"sniffer"`
	DNS               *DNSOptions        `json:"dns"`
	Geo               *GeoOptions        `json:"geo"`
	Tun               *TunOptions        `json:"tun"`
	Inbound           *InboundOptions    `json:"inbound"`
	Controller        *ControllerOptions `json:"controller"`
}

func coreOptionsValue(value *CoreOptions) CoreOptions {
	if value == nil {
		return CoreOptions{}
	}
	return *value
}
func validateCoreOptions(v CoreOptions) error {
	if v.LogLevel != nil {
		switch *v.LogLevel {
		case "silent", "error", "warning", "info", "debug":
		default:
			return errors.New("日志级别无效")
		}
	}
	if v.FindProcessMode != nil {
		switch *v.FindProcessMode {
		case "off", "strict", "always":
		default:
			return errors.New("进程匹配模式无效")
		}
	}
	if v.KeepAliveInterval != nil && (*v.KeepAliveInterval < 1 || *v.KeepAliveInterval > 86400) {
		return errors.New("连接保活间隔必须在 1 到 86400 秒之间")
	}
	if v.DNS != nil {
		if err := validateDNSOptions(*v.DNS); err != nil {
			return err
		}
	}
	if v.Geo != nil {
		if err := validateGeoOptions(*v.Geo); err != nil {
			return err
		}
	}
	if v.Tun != nil {
		if err := validateTunOptions(*v.Tun); err != nil {
			return err
		}
	}
	if v.Inbound != nil {
		if err := validateInboundOptions(*v.Inbound, 0); err != nil {
			return err
		}
	}
	if v.Controller != nil {
		if err := validateControllerOptions(*v.Controller); err != nil {
			return err
		}
	}
	return nil
}
func runtimeConfigWithOptions(content string, mixedPort, controllerPort int, secret, mode string, lan LANSettings, tun bool, options CoreOptions) ([]byte, error) {
	if err := validateCoreOptions(options); err != nil {
		return nil, err
	}
	data, err := runtimeConfigWithTakeovers(content, mixedPort, controllerPort, secret, mode, lan, tun)
	if err != nil {
		return nil, err
	}
	cfg, err := unmarshalConfigMap(data)
	if err != nil {
		return nil, err
	}
	if options.LogLevel != nil {
		cfg["log-level"] = *options.LogLevel
	}
	if options.IPv6 != nil {
		cfg["ipv6"] = *options.IPv6
	}
	if options.TCPConcurrent != nil {
		cfg["tcp-concurrent"] = *options.TCPConcurrent
	}
	if options.UnifiedDelay != nil {
		cfg["unified-delay"] = *options.UnifiedDelay
	}
	if options.FindProcessMode != nil {
		cfg["find-process-mode"] = *options.FindProcessMode
	}
	if options.KeepAliveInterval != nil {
		cfg["keep-alive-interval"] = *options.KeepAliveInterval
	}
	if options.Sniffer != nil {
		section, ok := cfg["sniffer"].(map[string]any)
		if !ok && cfg["sniffer"] != nil {
			return nil, errors.New("配置中的 sniffer 必须为对象")
		}
		if section == nil {
			section = map[string]any{}
		}
		section["enable"] = *options.Sniffer
		if *options.Sniffer {
			fillSnifferDefaults(section)
		}
		cfg["sniffer"] = section
	}
	if options.DNS != nil {
		if err := mergeDNSOptions(cfg, *options.DNS); err != nil {
			return nil, err
		}
	}
	if options.Geo != nil {
		if options.Geo.DataMode != nil {
			cfg["geodata-mode"] = *options.Geo.DataMode
		}
		if options.Geo.AutoUpdate != nil {
			cfg["geo-auto-update"] = *options.Geo.AutoUpdate
		}
		if options.Geo.UpdateInterval != nil {
			cfg["geo-update-interval"] = *options.Geo.UpdateInterval
		}
	}
	if tun && options.Tun != nil {
		mergeTunOptions(cfg["tun"].(map[string]any), *options.Tun)
	}
	if tun {
		if err := ensureTunDNS(cfg); err != nil {
			return nil, err
		}
		if err := ensureTunSniffer(cfg, options.Sniffer); err != nil {
			return nil, err
		}
	}
	if dns, ok := cfg["dns"].(map[string]any); ok && dns["enable"] == true {
		fillDNSDefaults(dns)
	}
	if options.Inbound != nil {
		if err := validateInboundOptions(*options.Inbound, mixedPort); err != nil {
			return nil, err
		}
		cfg["port"] = options.Inbound.HTTPPort
		cfg["socks-port"] = options.Inbound.SOCKSPort
		if options.Inbound.Authentication != nil {
			cfg["authentication"] = *options.Inbound.Authentication
		}
	}
	if options.Controller != nil {
		cfg["external-controller"] = net.JoinHostPort(controllerBind(*options.Controller), strconv.Itoa(controllerPort))
	}
	if err := validateCustomListeners(cfg, mixedPort, controllerPort, options.Inbound); err != nil {
		return nil, err
	}
	// macOS system proxy and the app's own checks connect over loopback without credentials.
	if auth, ok := cfg["authentication"].([]any); ok && len(auth) > 0 {
		ensureLoopbackAuth(cfg)
	}
	if auth, ok := cfg["authentication"].([]string); ok && len(auth) > 0 {
		ensureLoopbackAuth(cfg)
	}
	return yaml.Marshal(cfg)
}
func (p *Profiles) CoreSettings() (CoreOptions, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return CoreOptions{}, err
	}
	return coreOptionsValue(d.CoreOptions), nil
}
func (s *Service) applyCoreOptions(options CoreOptions) (bool, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil {
		return false, nil
	}
	mode := currentInProcessMode()
	next, err := runtimeConfigWithOptions(s.currentContent, s.mixedPort, s.port, s.secret, mode, s.lan, s.useTun, options)
	if err != nil {
		return true, err
	}
	previous, err := runtimeConfigWithOptions(s.currentContent, s.mixedPort, s.port, s.secret, mode, s.lan, s.useTun, s.coreOptions)
	if err != nil {
		return true, err
	}
	if string(next) == string(previous) {
		s.coreOptions = options
		return true, nil
	}
	if err = s.markGeoAuto(next); err != nil {
		return true, err
	}
	if err = s.applyRuntimeConfigLocked(next); err != nil {
		if rollback := s.applyRuntimeConfigLocked(previous); rollback != nil {
			return true, errors.Join(fmt.Errorf("应用内核设置失败：%w", err), fmt.Errorf("恢复旧设置失败：%w", rollback))
		}
		return true, fmt.Errorf("应用内核设置失败：%w", err)
	}
	s.coreOptions = options
	return true, nil
}
func (p *Profiles) SetCoreSettings(value CoreOptions) (CoreOptions, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return CoreOptions{}, err
	}
	value.DNS = coreOptionsValue(d.CoreOptions).DNS
	value.Geo = coreOptionsValue(d.CoreOptions).Geo
	value.Tun = coreOptionsValue(d.CoreOptions).Tun
	value.Inbound = coreOptionsValue(d.CoreOptions).Inbound
	value.Controller = coreOptionsValue(d.CoreOptions).Controller
	return p.setCoreOptionsLocked(d, value)
}

func (p *Profiles) setCoreOptionsLocked(d profileData, value CoreOptions) (CoreOptions, error) {
	if err := validateCoreOptions(value); err != nil {
		return CoreOptions{}, err
	}
	previous := coreOptionsValue(d.CoreOptions)
	running := false
	var err error
	if p.service != nil {
		running, err = p.service.applyCoreOptions(value)
		if err != nil {
			return CoreOptions{}, err
		}
	}
	d.CoreOptions = &value
	if err = p.write(d); err != nil {
		if running {
			if _, rollback := p.service.applyCoreOptions(previous); rollback != nil {
				return CoreOptions{}, errors.Join(fmt.Errorf("保存设置失败：%w", err), fmt.Errorf("恢复旧设置失败：%w", rollback))
			}
		}
		return CoreOptions{}, err
	}
	return value, nil
}
