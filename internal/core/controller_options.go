package core

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"unicode"
)

type ControllerOptions struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
	Secret  string `json:"secret"`
}

func controllerBind(v ControllerOptions) string {
	if v.Address == "" || v.Address == "localhost" {
		return "127.0.0.1"
	}
	if v.Address == "*" {
		return "0.0.0.0"
	}
	return v.Address
}
func validateControllerOptions(v ControllerOptions) error {
	ip := net.ParseIP(controllerBind(v))
	if ip == nil {
		return errors.New("控制接口地址必须为本机 IP 地址或 *")
	}
	if v.Port < 0 || v.Port > 65535 {
		return errors.New("控制接口端口必须为 0 到 65535，0 表示自动选择")
	}
	if len(v.Secret) > 4096 || strings.IndexFunc(v.Secret, unicode.IsControl) >= 0 {
		return errors.New("控制接口密钥包含无效字符或过长")
	}
	if !ip.IsLoopback() && v.Secret == "" {
		return errors.New("向本机回环地址以外开放控制接口时，必须设置密钥")
	}
	return nil
}
func validateControllerPorts(value ControllerOptions, mixed int, inbound *InboundOptions) error {
	if value.Port == 0 {
		return nil
	}
	if value.Port == mixed || (inbound != nil && (value.Port == inbound.HTTPPort || value.Port == inbound.SOCKSPort)) {
		return errors.New("控制接口端口不能与代理入站端口相同")
	}
	return nil
}
func (s *Service) controllerAuthority() string {
	if s.controllerAddress != "" {
		return s.controllerAddress
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(s.port))
}
func (p *Profiles) ControllerSettings() (ControllerOptions, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return ControllerOptions{}, err
	}
	v := coreOptionsValue(d.CoreOptions).Controller
	if v == nil {
		return ControllerOptions{Address: "127.0.0.1"}, nil
	}
	return *v, nil
}
func (p *Profiles) SetControllerSettings(v ControllerOptions) (ControllerOptions, error) {
	v.Address = controllerBind(v)
	if err := validateControllerOptions(v); err != nil {
		return ControllerOptions{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.service != nil {
		p.service.opMu.Lock()
		defer p.service.opMu.Unlock()
		p.service.mu.Lock()
		defer p.service.mu.Unlock()
		if p.service.cmd != nil {
			return ControllerOptions{}, errors.New("请先停止内核，再修改控制接口")
		}
	}
	d, err := p.load()
	if err != nil {
		return ControllerOptions{}, err
	}
	options := coreOptionsValue(d.CoreOptions)
	if err = validateControllerPorts(v, d.MixedPort, options.Inbound); err != nil {
		return ControllerOptions{}, err
	}
	options.Controller = &v
	d.CoreOptions = &options
	if err = p.write(d); err != nil {
		return ControllerOptions{}, err
	}
	return v, nil
}
