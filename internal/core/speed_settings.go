package core

import (
	"errors"
	"net/url"
	"strings"
)

// SpeedSettings controls tests initiated by RocketScience, not subscription health checks.
type SpeedSettings struct {
	URL       string `json:"url"`
	TimeoutMS int    `json:"timeoutMs"`
}

func defaultSpeedSettings() SpeedSettings {
	return SpeedSettings{URL: "https://www.gstatic.com/generate_204", TimeoutMS: 5000}
}

func validateSpeedSettings(v SpeedSettings) (SpeedSettings, error) {
	v.URL = strings.TrimSpace(v.URL)
	u, err := url.Parse(v.URL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" {
		return v, errors.New("测速地址必须是完整的 HTTP 或 HTTPS 地址，不能包含账号或片段")
	}
	if v.TimeoutMS < 100 || v.TimeoutMS > 30000 {
		return v, errors.New("测速超时必须为 100 到 30000 毫秒")
	}
	return v, nil
}

func (p *Profiles) SpeedSettings() (SpeedSettings, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return SpeedSettings{}, err
	}
	if d.Speed == nil {
		return defaultSpeedSettings(), nil
	}
	return validateSpeedSettings(*d.Speed)
}

func (p *Profiles) SetSpeedSettings(v SpeedSettings) (SpeedSettings, error) {
	v, err := validateSpeedSettings(v)
	if err != nil {
		return SpeedSettings{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return SpeedSettings{}, err
	}
	d.Speed = &v
	if err = p.write(d); err != nil {
		return SpeedSettings{}, err
	}
	return v, nil
}
