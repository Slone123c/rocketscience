package core

import (
	"errors"
	"strings"
	"unicode"
)

// ProxyBypassSettings preserves the system's current exceptions unless Custom is enabled.
type ProxyBypassSettings struct {
	Custom  bool     `json:"custom"`
	Entries []string `json:"entries"`
}

func cloneBypass(value *[]string) *[]string {
	if value == nil {
		return nil
	}
	copy := append([]string{}, (*value)...)
	return &copy
}

func normalizeBypass(entries []string) ([]string, error) {
	if len(entries) > 200 {
		return nil, errors.New("最多允许 200 条代理例外")
	}
	result := make([]string, 0, len(entries))
	seen := map[string]bool{}
	for _, raw := range entries {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if len(v) > 255 || strings.Contains(v, "://") {
			return nil, errors.New("请输入域名、通配符或 IP 地址，每行一条")
		}
		for _, r := range v {
			if !(unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(".-_*:/", r)) {
				return nil, errors.New("例外地址包含不支持的字符")
			}
		}
		key := strings.ToLower(v)
		if !seen[key] {
			seen[key] = true
			result = append(result, v)
		}
	}
	return result, nil
}

func (p *Profiles) ProxyBypass() (ProxyBypassSettings, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return ProxyBypassSettings{}, err
	}
	if d.ProxyBypass == nil {
		return ProxyBypassSettings{Entries: []string{}}, nil
	}
	return ProxyBypassSettings{Custom: true, Entries: append([]string{}, (*d.ProxyBypass)...)}, nil
}

func (p *Profiles) SetProxyBypass(value ProxyBypassSettings) (ProxyBypassSettings, error) {
	entries, err := normalizeBypass(value.Entries)
	if err != nil {
		return ProxyBypassSettings{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.service != nil {
		p.service.opMu.Lock()
		defer p.service.opMu.Unlock()
		p.service.mu.Lock()
		defer p.service.mu.Unlock()
		if p.service.cmd != nil {
			return ProxyBypassSettings{}, errors.New("请先停止内核，再修改代理例外")
		}
	}
	d, err := p.load()
	if err != nil {
		return ProxyBypassSettings{}, err
	}
	if value.Custom {
		d.ProxyBypass = &entries
	} else {
		d.ProxyBypass = nil
		entries = []string{}
	}
	if err = p.write(d); err != nil {
		return ProxyBypassSettings{}, err
	}
	return ProxyBypassSettings{Custom: value.Custom, Entries: entries}, nil
}
