package core

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

type InboundOptions struct {
	HTTPPort       int       `json:"httpPort"`
	SOCKSPort      int       `json:"socksPort"`
	Authentication *[]string `json:"authentication"`
}

func validateInboundOptions(v InboundOptions, mixedPort int) error {
	for _, port := range []int{v.HTTPPort, v.SOCKSPort} {
		if port < 0 || port > 65535 {
			return errors.New("入站端口必须为 0 到 65535，0 表示关闭")
		}
		if port > 0 && port == mixedPort {
			return errors.New("独立入站端口不能与混合端口相同")
		}
	}
	if v.HTTPPort > 0 && v.HTTPPort == v.SOCKSPort {
		return errors.New("HTTP 和 SOCKS 端口不能相同")
	}
	if v.Authentication != nil {
		if len(*v.Authentication) > 100 {
			return errors.New("最多允许 100 个代理账号")
		}
		users := map[string]bool{}
		for _, entry := range *v.Authentication {
			username, password, ok := strings.Cut(entry, ":")
			if !ok || username == "" || password == "" || len(entry) > 2048 || strings.IndexFunc(entry, unicode.IsControl) >= 0 {
				return errors.New("代理账号需包含非空的用户名和密码")
			}
			if users[username] {
				return errors.New("代理用户名不能重复")
			}
			users[username] = true
		}
	}
	return nil
}
func ensureLoopbackAuth(cfg map[string]any) {
	prefixes, _ := cfg["skip-auth-prefixes"].([]any)
	for _, required := range []string{"127.0.0.0/8", "::1/128"} {
		found := false
		for _, prefix := range prefixes {
			if prefix == required {
				found = true
				break
			}
		}
		if !found {
			prefixes = append(prefixes, required)
		}
	}
	cfg["skip-auth-prefixes"] = prefixes
}

func validateCustomListeners(cfg map[string]any, mixedPort, controllerPort int, inbound *InboundOptions) error {
	raw, ok := cfg["listeners"]
	if !ok || raw == nil {
		return nil
	}
	listeners, ok := raw.([]any)
	if !ok {
		return errors.New("配置中的 listeners 必须为列表")
	}
	for _, entry := range listeners {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		if name == "rocketscience-lan" {
			continue
		}
		port := listenerPort(m["port"])
		if port <= 0 {
			continue
		}
		if port == mixedPort {
			return fmt.Errorf("配置中的自定义监听器 %s 占用了混合端口 %d", name, port)
		}
		if controllerPort > 0 && port == controllerPort {
			return fmt.Errorf("配置中的自定义监听器 %s 占用了控制接口端口 %d", name, port)
		}
		if inbound != nil && (port == inbound.HTTPPort || port == inbound.SOCKSPort) {
			return fmt.Errorf("配置中的自定义监听器 %s 占用了独立入站端口 %d", name, port)
		}
	}
	return nil
}

func listenerPort(v any) int {
	switch p := v.(type) {
	case int:
		return p
	case int64:
		return int(p)
	case uint64:
		return int(p)
	case *yaml.Node:
		var n int
		if p != nil && p.Decode(&n) == nil {
			return n
		}
	}
	return 0
}

func (p *Profiles) InboundSettings() (InboundOptions, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return InboundOptions{}, err
	}
	options := coreOptionsValue(d.CoreOptions)
	if options.Inbound == nil {
		return InboundOptions{}, nil
	}
	return *options.Inbound, nil
}
func (p *Profiles) SetInboundSettings(value InboundOptions) (InboundOptions, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.service != nil {
		p.service.opMu.Lock()
		defer p.service.opMu.Unlock()
		p.service.mu.Lock()
		defer p.service.mu.Unlock()
		if p.service.cmd != nil {
			return InboundOptions{}, errors.New("请先停止内核，再修改入站端口或代理认证")
		}
	}
	d, err := p.load()
	if err != nil {
		return InboundOptions{}, err
	}
	if err = validateInboundOptions(value, d.MixedPort); err != nil {
		return InboundOptions{}, err
	}
	options := coreOptionsValue(d.CoreOptions)
	if options.Controller != nil {
		if err = validateControllerPorts(*options.Controller, d.MixedPort, &value); err != nil {
			return InboundOptions{}, err
		}
	}
	options.Inbound = &value
	d.CoreOptions = &options
	if err = p.write(d); err != nil {
		return InboundOptions{}, err
	}
	return value, nil
}
func portUnavailableError(port int, cause error) error {
	owner := ""
	out, _ := exec.Command("/usr/sbin/lsof", "-nP", "-iTCP:"+strconv.Itoa(port), "-sTCP:LISTEN", "-Fc").Output()
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "c") && len(line) > 1 {
			owner = "，占用程序：" + strings.TrimPrefix(line, "c")
			break
		}
	}
	suggestion := ""
	if listener, err := net.Listen("tcp", "127.0.0.1:0"); err == nil {
		suggestion = fmt.Sprintf("；可尝试 %d", listener.Addr().(*net.TCPAddr).Port)
		_ = listener.Close()
	}
	return fmt.Errorf("%d 端口不可用%s%s：%w", port, owner, suggestion, cause)
}
