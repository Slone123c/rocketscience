package core

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"unicode"
)

// Nil fields inherit their values from the selected configuration.
type DNSOptions struct {
	Enabled            *bool     `json:"enabled"`
	IPv6               *bool     `json:"ipv6"`
	EnhancedMode       *string   `json:"enhancedMode"`
	FakeIPRange        *string   `json:"fakeIPRange"`
	DefaultNameservers *[]string `json:"defaultNameservers"`
	Nameservers        *[]string `json:"nameservers"`
	Fallback           *[]string `json:"fallback"`
}

func validateDNSServers(entries *[]string, field string, requireIP bool, allowEmpty bool) error {
	if entries == nil {
		return nil
	}
	if !allowEmpty && len(*entries) == 0 {
		return fmt.Errorf("%s 不能为空，请提供地址或选择跟随配置", field)
	}
	if len(*entries) > 50 {
		return fmt.Errorf("%s 最多允许 50 个地址", field)
	}
	for _, v := range *entries {
		if len(v) == 0 || len(v) > 2048 || strings.TrimSpace(v) != v || strings.IndexFunc(v, unicode.IsSpace) >= 0 || strings.IndexFunc(v, unicode.IsControl) >= 0 {
			return fmt.Errorf("%s 包含无效地址", field)
		}
		if requireIP {
			plain := strings.SplitN(v, "#", 2)[0]
			var host string
			if strings.Contains(plain, "://") {
				u, err := url.Parse(plain)
				if err != nil || u.Hostname() == "" || u.User != nil {
					return fmt.Errorf("%s 必须使用 IP 地址", field)
				}
				host = u.Hostname()
			} else {
				host = plain
				if h, _, err := net.SplitHostPort(plain); err == nil {
					host = h
				}
			}
			if net.ParseIP(host) == nil {
				return fmt.Errorf("%s 必须使用 IP 地址", field)
			}
		}
	}
	return nil
}
func validateDNSOptions(v DNSOptions) error {
	if v.EnhancedMode != nil && *v.EnhancedMode != "fake-ip" && *v.EnhancedMode != "redir-host" {
		return errors.New("DNS 处理模式无效")
	}
	if v.FakeIPRange != nil {
		ip, _, err := net.ParseCIDR(*v.FakeIPRange)
		if err != nil || ip.To4() == nil {
			return errors.New("Fake-IP 网段必须为有效的 IPv4 CIDR")
		}
	}
	if err := validateDNSServers(v.DefaultNameservers, "默认 DNS", true, false); err != nil {
		return err
	}
	if err := validateDNSServers(v.Nameservers, "上游 DNS", false, false); err != nil {
		return err
	}
	return validateDNSServers(v.Fallback, "备用 DNS", false, true)
}
func mergeDNSOptions(cfg map[string]any, value DNSOptions) error {
	dns, ok := cfg["dns"].(map[string]any)
	if !ok && cfg["dns"] != nil {
		return errors.New("配置中的 dns 必须为对象")
	}
	if dns == nil {
		dns = map[string]any{}
	}
	if value.Enabled != nil {
		dns["enable"] = *value.Enabled
	}
	if value.IPv6 != nil {
		dns["ipv6"] = *value.IPv6
	}
	if value.EnhancedMode != nil {
		dns["enhanced-mode"] = *value.EnhancedMode
	}
	if value.FakeIPRange != nil {
		dns["fake-ip-range"] = *value.FakeIPRange
	}
	if value.DefaultNameservers != nil {
		dns["default-nameserver"] = *value.DefaultNameservers
	}
	if value.Nameservers != nil {
		dns["nameserver"] = *value.Nameservers
	}
	if value.Fallback != nil {
		dns["fallback"] = *value.Fallback
	}
	if len(dns) > 0 {
		cfg["dns"] = dns
	}
	return nil
}
func (p *Profiles) DNSSettings() (DNSOptions, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return DNSOptions{}, err
	}
	options := coreOptionsValue(d.CoreOptions)
	if options.DNS == nil {
		return DNSOptions{}, nil
	}
	return *options.DNS, nil
}
func (p *Profiles) SetDNSSettings(value DNSOptions) (DNSOptions, error) {
	if err := validateDNSOptions(value); err != nil {
		return DNSOptions{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return DNSOptions{}, err
	}
	options := coreOptionsValue(d.CoreOptions)
	options.DNS = &value
	content := activeContent(d, p.draft)
	if content == "" {
		content = "rules:\n  - MATCH,DIRECT\n"
	} else {
		var mergeErr error
		content, _, _, _, mergeErr = mergeOverrides(content, d.Overrides)
		if mergeErr != nil {
			return DNSOptions{}, mergeErr
		}
	}
	candidate, err := runtimeConfigWithOptions(content, d.MixedPort, 19090, "validation", "", lanValue(d.LAN), d.Tun, options)
	if err != nil {
		return DNSOptions{}, err
	}
	if err = validateProfile(string(candidate)); err != nil {
		return DNSOptions{}, fmt.Errorf("DNS 设置无法应用：%w", err)
	}
	if _, err = p.setCoreOptionsLocked(d, options); err != nil {
		return DNSOptions{}, err
	}
	return value, nil
}
