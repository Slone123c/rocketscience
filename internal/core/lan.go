package core

import (
	"errors"
	"fmt"
	"net"
	"sort"
)

type LANSettings struct {
	Enabled bool   `json:"enabled"`
	Address string `json:"address"`
}
type LANInterface struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

func lanValue(value *LANSettings) LANSettings {
	if value == nil {
		return LANSettings{Address: "*"}
	}
	return *value
}
func (p *Profiles) LANInterfaces() ([]LANInterface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	result := []LANInterface{}
	for _, item := range interfaces {
		if item.Flags&net.FlagUp == 0 || item.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := item.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err != nil || ip.To4() == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			result = append(result, LANInterface{Name: item.Name, Address: ip.String()})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name == result[j].Name {
			return result[i].Address < result[j].Address
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}
func validateLAN(value LANSettings) error {
	if !value.Enabled {
		return nil
	}
	if value.Address == "*" {
		return nil
	}
	ip := net.ParseIP(value.Address)
	if ip == nil || ip.To4() == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() {
		return errors.New("请选择可用的局域网 IPv4 地址")
	}
	list, err := (&Profiles{}).LANInterfaces()
	if err != nil {
		return err
	}
	for _, entry := range list {
		if entry.Address == ip.String() {
			return nil
		}
	}
	return fmt.Errorf("局域网地址 %s 已不可用，请重新选择网卡", value.Address)
}
func (p *Profiles) LANSettings() (LANSettings, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return LANSettings{}, err
	}
	return lanValue(d.LAN), nil
}
func (p *Profiles) SetLANSettings(value LANSettings) (LANSettings, error) {
	if value.Address == "" {
		value.Address = "*"
	}
	if err := validateLAN(value); err != nil {
		return LANSettings{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.service != nil {
		p.service.opMu.Lock()
		defer p.service.opMu.Unlock()
		p.service.mu.Lock()
		defer p.service.mu.Unlock()
		if p.service.cmd != nil {
			return LANSettings{}, errors.New("请先停止内核，再修改局域网共享")
		}
	}
	d, err := p.load()
	if err != nil {
		return LANSettings{}, err
	}
	d.LAN = &value
	if err = p.write(d); err != nil {
		return LANSettings{}, err
	}
	return value, nil
}
