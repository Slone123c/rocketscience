package core

import (
	"errors"
	"net"
	"strconv"
	"strings"
)

// Nil values use RocketScience's TUN defaults. Enable is controlled separately.
type TunOptions struct {
	Stack               *string   `json:"stack"`
	AutoRoute           *bool     `json:"autoRoute"`
	AutoDetectInterface *bool     `json:"autoDetectInterface"`
	StrictRoute         *bool     `json:"strictRoute"`
	MTU                 *int      `json:"mtu"`
	DNSHijack           *[]string `json:"dnsHijack"`
}

func validateTunOptions(value TunOptions) error {
	if value.Stack != nil {
		switch *value.Stack {
		case "system", "gvisor", "mixed":
		default:
			return errors.New("TUN 网络栈无效")
		}
	}
	if value.MTU != nil && (*value.MTU < 576 || *value.MTU > 65535) {
		return errors.New("TUN MTU 必须为 576 到 65535 之间的整数")
	}
	if value.DNSHijack != nil {
		if len(*value.DNSHijack) > 64 {
			return errors.New("DNS 劫持列表最多允许 64 项")
		}
		for _, entry := range *value.DNSHijack {
			address := entry
			if strings.HasPrefix(address, "tcp://") {
				address = strings.TrimPrefix(address, "tcp://")
			} else if strings.HasPrefix(address, "udp://") {
				address = strings.TrimPrefix(address, "udp://")
			}
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return errors.New("DNS 劫持地址需为 any:53、IP:端口或 tcp://IP:端口")
			}
			number, err := strconv.Atoi(port)
			if err != nil || number < 1 || number > 65535 || (host != "any" && net.ParseIP(host) == nil) {
				return errors.New("DNS 劫持地址或端口无效")
			}
		}
	}
	return nil
}
func mergeTunOptions(tun map[string]any, value TunOptions) {
	if value.Stack != nil {
		tun["stack"] = *value.Stack
	}
	if value.AutoRoute != nil {
		tun["auto-route"] = *value.AutoRoute
	}
	if value.AutoDetectInterface != nil {
		tun["auto-detect-interface"] = *value.AutoDetectInterface
	}
	if value.StrictRoute != nil {
		tun["strict-route"] = *value.StrictRoute
	}
	if value.MTU != nil {
		tun["mtu"] = *value.MTU
	}
	if value.DNSHijack != nil {
		tun["dns-hijack"] = *value.DNSHijack
	}
}
func (p *Profiles) TunSettings() (TunOptions, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return TunOptions{}, err
	}
	options := coreOptionsValue(d.CoreOptions)
	if options.Tun == nil {
		return TunOptions{}, nil
	}
	return *options.Tun, nil
}
func (p *Profiles) SetTunSettings(value TunOptions) (TunOptions, error) {
	if err := validateTunOptions(value); err != nil {
		return TunOptions{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.service != nil {
		p.service.opMu.Lock()
		defer p.service.opMu.Unlock()
		p.service.mu.Lock()
		defer p.service.mu.Unlock()
		if p.service.cmd != nil {
			return TunOptions{}, errors.New("请先停止内核，再修改 TUN 高级参数")
		}
	}
	d, err := p.load()
	if err != nil {
		return TunOptions{}, err
	}
	options := coreOptionsValue(d.CoreOptions)
	options.Tun = &value
	d.CoreOptions = &options
	if err = p.write(d); err != nil {
		return TunOptions{}, err
	}
	return value, nil
}
