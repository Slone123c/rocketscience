package core

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// unmarshalConfigMap decodes a YAML mapping like yaml.Unmarshal into map[string]any,
// while preserving raw *yaml.Node subtrees for nested mappings whose key order matters
// (e.g. hosts, dns.nameserver-policy) and keeping numeric-looking scalars whose original
// text would be altered by canonical re-serialization (e.g. short-id 01234567, 1234e5, 0x12).
func unmarshalConfigMap(data []byte) (map[string]any, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 {
		return nil, nil
	}
	root := doc.Content[0]
	if root.Kind == yaml.AliasNode {
		root = root.Alias
	}
	if root.Kind != yaml.MappingNode {
		var cfg map[string]any
		if err := root.Decode(&cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}
	v, err := nodeToAny(root, "")
	if err != nil {
		return nil, err
	}
	return v.(map[string]any), nil
}

func nodeToAny(n *yaml.Node, path string) (any, error) {
	switch n.Kind {
	case yaml.AliasNode:
		return nodeToAny(n.Alias, path)
	case yaml.MappingNode:
		if path != "" && path != "sniffer" && path != "dns" && path != "tun" && path != "listeners[]" {
			return resolveSubtree(n)
		}
		out := make(map[string]any, len(n.Content)/2)
		// Merge keys first so explicit keys override them, as yaml.v3 does.
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Tag != "!!merge" {
				continue
			}
			sources := []*yaml.Node{n.Content[i+1]}
			if sources[0].Kind == yaml.AliasNode && sources[0].Alias != nil {
				sources[0] = sources[0].Alias
			}
			if sources[0].Kind == yaml.SequenceNode {
				sources = sources[0].Content
			}
			for j := len(sources) - 1; j >= 0; j-- {
				src, err := nodeToAny(sources[j], path)
				if err != nil {
					return nil, err
				}
				m, ok := src.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("line %d: 合并键必须引用映射", n.Line)
				}
				for k, val := range m {
					out[k] = val
				}
			}
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Tag == "!!merge" {
				continue
			}
			if k.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("line %d: 不支持非字符串键", k.Line)
			}
			childPath := k.Value
			if path != "" {
				childPath = path + "." + k.Value
			}
			val, err := nodeToAny(n.Content[i+1], childPath)
			if err != nil {
				return nil, err
			}
			out[k.Value] = val
		}
		return out, nil
	case yaml.SequenceNode:
		out := make([]any, 0, len(n.Content))
		elemPath := path + "[]"
		for _, c := range n.Content {
			val, err := nodeToAny(c, elemPath)
			if err != nil {
				return nil, err
			}
			out = append(out, val)
		}
		return out, nil
	}
	var val any
	if err := n.Decode(&val); err != nil {
		return nil, err
	}
	if n.Tag == "!!int" || n.Tag == "!!float" {
		if canonical, err := yaml.Marshal(val); err == nil && string(canonical) != n.Value+"\n" {
			// Re-emit the original plain spelling: typed int fields still parse it,
			// and string fields (short-id, uuid) receive the exact original text.
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: n.Tag, Value: n.Value}, nil
		}
	}
	return val, nil
}

// resolveSubtree clones a preserved *yaml.Node subtree while expanding aliases and << merges
// in place, because top-level anchor keys may be dropped when the top-level map is re-serialized.
func resolveSubtree(n *yaml.Node) (*yaml.Node, error) {
	if n == nil {
		return nil, nil
	}
	if n.Kind == yaml.AliasNode {
		return resolveSubtree(n.Alias)
	}
	clone := *n
	clone.Anchor = ""
	switch n.Kind {
	case yaml.MappingNode:
		var pairs []*yaml.Node
		seen := make(map[string]int)
		setPair := func(k, v *yaml.Node) {
			if idx, ok := seen[k.Value]; ok {
				pairs[idx+1] = v
				return
			}
			seen[k.Value] = len(pairs)
			pairs = append(pairs, k, v)
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Tag != "!!merge" {
				continue
			}
			target := n.Content[i+1]
			if target.Kind == yaml.AliasNode && target.Alias != nil {
				target = target.Alias
			}
			sources := []*yaml.Node{target}
			if target.Kind == yaml.SequenceNode {
				sources = target.Content
			}
			for j := len(sources) - 1; j >= 0; j-- {
				src, err := resolveSubtree(sources[j])
				if err != nil {
					return nil, err
				}
				if src.Kind != yaml.MappingNode {
					return nil, fmt.Errorf("line %d: 合并键必须引用映射", n.Line)
				}
				for m := 0; m+1 < len(src.Content); m += 2 {
					setPair(src.Content[m], src.Content[m+1])
				}
			}
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Tag == "!!merge" {
				continue
			}
			if k.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("line %d: 不支持非字符串键", n.Line)
			}
			keyClone := *k
			keyClone.Anchor = ""
			valClone, err := resolveSubtree(n.Content[i+1])
			if err != nil {
				return nil, err
			}
			setPair(&keyClone, valClone)
		}
		clone.Content = pairs
	case yaml.SequenceNode:
		clone.Content = make([]*yaml.Node, len(n.Content))
		for i, c := range n.Content {
			elem, err := resolveSubtree(c)
			if err != nil {
				return nil, err
			}
			clone.Content[i] = elem
		}
	}
	return &clone, nil
}
