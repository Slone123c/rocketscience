package core

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var reservedRuntimeFiles = map[string]bool{
	"config.yaml":  true,
	"cache.db":     true,
	"heartbeat":    true,
	"tun-core.log": true,
	"geoip.metadb": true,
	"geoip.dat":    true,
	"geosite.dat":  true,
	"country.mmdb": true,
	"asn.mmdb":     true,
}

func validateProviderPaths(cfg map[string]any) error {
	if err := validateNamedEntries(cfg); err != nil {
		return err
	}
	seen := map[string]string{}
	for _, section := range []string{"proxy-providers", "rule-providers"} {
		rawSection, ok := cfg[section]
		if !ok || rawSection == nil {
			continue
		}
		if err := checkProviderSection(section, rawSection, seen); err != nil {
			return err
		}
	}
	return nil
}

func validateNamedEntries(cfg map[string]any) error {
	for _, section := range []string{"proxies", "proxy-groups"} {
		items, ok := cfg[section].([]any)
		if !ok {
			continue
		}
		for _, item := range items {
			switch v := item.(type) {
			case *yaml.Node:
				if v.Kind == yaml.AliasNode && v.Alias != nil {
					v = v.Alias
				}
				if v.Kind != yaml.MappingNode {
					continue
				}
				nameFound := false
				for i := 0; i+1 < len(v.Content); i += 2 {
					if v.Content[i].Value == "name" {
						nameFound = true
						if strings.TrimSpace(v.Content[i+1].Value) == "" {
							return fmt.Errorf("配置中的 %s 节点或策略组名称不能为空", section)
						}
					}
				}
				if !nameFound {
					return fmt.Errorf("配置中的 %s 节点或策略组名称不能为空", section)
				}
			case map[string]any:
				rawName, exists := v["name"]
				nameStr, ok := rawName.(string)
				if !exists || !ok || strings.TrimSpace(nameStr) == "" {
					return fmt.Errorf("配置中的 %s 节点或策略组名称不能为空", section)
				}
			}
		}
	}
	return nil
}

func checkProviderSection(section string, rawSection any, seen map[string]string) error {
	switch s := rawSection.(type) {
	case *yaml.Node:
		if s.Kind == yaml.AliasNode && s.Alias != nil {
			s = s.Alias
		}
		if s.Kind != yaml.MappingNode {
			return nil
		}
		for i := 0; i+1 < len(s.Content); i += 2 {
			name := s.Content[i].Value
			entry := s.Content[i+1]
			if entry.Kind == yaml.AliasNode && entry.Alias != nil {
				entry = entry.Alias
			}
			if entry.Kind != yaml.MappingNode {
				continue
			}
			for j := 0; j+1 < len(entry.Content); j += 2 {
				if entry.Content[j].Value == "path" {
					if err := checkProviderPath(section+"."+name, entry.Content[j+1].Value, seen); err != nil {
						return err
					}
				}
			}
		}
	case map[string]any:
		for name, rawEntry := range s {
			entry, ok := rawEntry.(map[string]any)
			if !ok {
				continue
			}
			if rawPath, hasPath := entry["path"]; hasPath && rawPath != nil {
				pathStr, ok := rawPath.(string)
				if !ok {
					return fmt.Errorf("配置中的 %s.%s 路径必须为字符串", section, name)
				}
				if err := checkProviderPath(section+"."+name, pathStr, seen); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func checkProviderPath(provider, raw string, seen map[string]string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Errorf("配置中的 %s 路径不能为空", provider)
	}
	if filepath.IsAbs(trimmed) || strings.HasPrefix(trimmed, "~") || strings.ContainsRune(trimmed, '\\') {
		return fmt.Errorf("配置中的 %s 路径 %q 不能使用绝对路径或工作目录外路径", provider, raw)
	}
	clean := filepath.Clean(trimmed)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("配置中的 %s 路径 %q 不能超出工作目录", provider, raw)
	}
	key := strings.ToLower(clean)
	if reservedRuntimeFiles[key] {
		return fmt.Errorf("配置中的 %s 路径 %q 与运行时保留文件冲突", provider, raw)
	}
	if prev, exists := seen[key]; exists {
		return fmt.Errorf("配置中的 %s 与 %s 使用了相同的缓存路径 %q", provider, prev, raw)
	}
	seen[key] = provider
	return nil
}

