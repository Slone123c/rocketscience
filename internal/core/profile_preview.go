package core

import (
	"errors"
	"strings"

	"gopkg.in/yaml.v3"
)

// PreviewGroups reads saved names and protocol metadata only. It never starts
// mihomo, fetches providers, or returns server addresses and credentials.
func (p *Profiles) PreviewGroups() ([]Proxy, error) {
	p.mu.Lock()
	d, err := p.load()
	p.mu.Unlock()
	if err != nil {
		return nil, err
	}
	for _, profile := range d.Profiles {
		if profile.ID == d.ActiveID {
			return previewProfileGroups(profile.Content)
		}
	}
	return []Proxy{}, nil
}

func previewProfileGroups(content string) ([]Proxy, error) {
	var config struct {
		Proxies []struct {
			Name string `yaml:"name"`
			Type string `yaml:"type"`
			UDP  *bool  `yaml:"udp"`
		} `yaml:"proxies"`
		Groups []struct {
			Name    string   `yaml:"name"`
			Type    string   `yaml:"type"`
			Proxies []string `yaml:"proxies"`
		} `yaml:"proxy-groups"`
	}
	if err := yaml.Unmarshal([]byte(content), &config); err != nil {
		return nil, errors.New("无法读取配置中的节点信息，请检查配置格式")
	}
	types := map[string]string{"select": "Selector", "url-test": "URLTest", "fallback": "Fallback", "load-balance": "LoadBalance", "relay": "Relay"}
	nodes := map[string]ProxyNode{"DIRECT": {Name: "DIRECT", Type: "Direct"}, "REJECT": {Name: "REJECT", Type: "Reject"}}
	for _, node := range config.Proxies {
		nodes[node.Name] = ProxyNode{Name: node.Name, Type: node.Type, UDP: node.UDP}
	}
	for _, group := range config.Groups {
		nodes[group.Name] = ProxyNode{Name: group.Name, Type: types[strings.ToLower(group.Type)]}
	}
	groups := []Proxy{}
	referenced := map[string]bool{}
	for _, group := range config.Groups {
		v := Proxy{Name: group.Name, Type: types[strings.ToLower(group.Type)], All: []string{}, Nodes: []ProxyNode{}}
		for _, name := range group.Proxies {
			node, ok := nodes[name]
			if !ok {
				node = ProxyNode{Name: name}
			}
			v.All = append(v.All, name)
			v.Nodes = append(v.Nodes, node)
			referenced[name] = true
		}
		groups = append(groups, v)
	}
	unassigned := Proxy{Name: "配置中的独立节点", Type: "Selector", All: []string{}, Nodes: []ProxyNode{}}
	for _, node := range config.Proxies {
		if !referenced[node.Name] {
			unassigned.All = append(unassigned.All, node.Name)
			unassigned.Nodes = append(unassigned.Nodes, nodes[node.Name])
		}
	}
	if len(unassigned.All) > 0 {
		groups = append(groups, unassigned)
	}
	return groups, nil
}
