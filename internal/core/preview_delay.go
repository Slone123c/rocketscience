package core

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/metacubex/mihomo/adapter"
	"gopkg.in/yaml.v3"
)

// TestPreview measures a group's nodes while the core is stopped. Each node in
// the saved profile is dialed directly from the app, so the result is what the
// node would answer after start. Nested groups, DIRECT, REJECT and provider
// nodes (loaded only after start) have no server here and report -1, which the
// page shows as untested rather than as a timeout.
func (p *Profiles) TestPreview(group, target string, timeoutMS int) (map[string]int, error) {
	if err := validateDelayTest(group, target, timeoutMS); err != nil {
		return nil, err
	}
	p.mu.Lock()
	if p.service != nil && p.service.Status().Running {
		p.mu.Unlock()
		return nil, errors.New("内核运行中，请直接测速")
	}
	d, err := p.load()
	p.mu.Unlock()
	if err != nil {
		return nil, err
	}
	content := ""
	for _, profile := range d.Profiles {
		if profile.ID == d.ActiveID {
			content = profile.Content
		}
	}
	if content == "" {
		return nil, errors.New("没有正在使用的配置")
	}
	return testPreviewGroup(content, group, target, timeoutMS)
}

func testPreviewGroup(content, group, target string, timeoutMS int) (map[string]int, error) {
	var config struct {
		Proxies []map[string]any `yaml:"proxies"`
		Groups  []struct {
			Name    string   `yaml:"name"`
			Proxies []string `yaml:"proxies"`
		} `yaml:"proxy-groups"`
	}
	if err := yaml.Unmarshal([]byte(content), &config); err != nil {
		return nil, errors.New("无法读取配置中的节点信息，请检查配置格式")
	}
	byName := map[string]map[string]any{}
	for _, node := range config.Proxies {
		if name, ok := node["name"].(string); ok {
			byName[name] = node
		}
	}
	members, found := []string(nil), false
	for _, g := range config.Groups {
		if g.Name == group {
			members, found = g.Proxies, true
		}
	}
	if !found && group == unassignedGroupName {
		groups, err := previewProfileGroups(content)
		if err != nil {
			return nil, err
		}
		for _, g := range groups {
			if g.Name == group {
				members, found = g.All, true
			}
		}
	}
	if !found {
		return nil, errors.New("测速失败：未找到策略组 " + group)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()
	result := map[string]int{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	slots := make(chan struct{}, 16)
	for _, name := range members {
		mapping, ok := byName[name]
		if !ok {
			mu.Lock()
			result[name] = -1
			mu.Unlock()
			continue
		}
		wg.Add(1)
		go func(name string, mapping map[string]any) {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			delay := 0
			if proxy, err := adapter.ParseProxy(mapping); err == nil {
				if ms, err := proxy.URLTest(ctx, target, nil); err == nil {
					delay = int(ms)
				}
				_ = proxy.Close()
			}
			mu.Lock()
			result[name] = delay
			mu.Unlock()
		}(name, mapping)
	}
	wg.Wait()
	return result, nil
}
