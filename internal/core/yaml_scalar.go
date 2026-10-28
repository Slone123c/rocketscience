package core

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// unmarshalConfigMap decodes a YAML mapping like yaml.Unmarshal into map[string]any,
// except that numeric-looking scalars which would not re-serialize to their original
// text (e.g. short-id 01234567, 1234e5, 0x12) are kept as raw nodes so the original
// spelling reaches the core unchanged.
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
	v, err := nodeToAny(root)
	if err != nil {
		return nil, err
	}
	return v.(map[string]any), nil
}

func nodeToAny(n *yaml.Node) (any, error) {
	switch n.Kind {
	case yaml.AliasNode:
		return nodeToAny(n.Alias)
	case yaml.MappingNode:
		out := make(map[string]any, len(n.Content)/2)
		// Merge keys first so explicit keys override them, as yaml.v3 does.
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Tag != "!!merge" {
				continue
			}
			sources := []*yaml.Node{n.Content[i+1]}
			if sources[0].Kind == yaml.SequenceNode {
				sources = sources[0].Content
			}
			for j := len(sources) - 1; j >= 0; j-- {
				src, err := nodeToAny(sources[j])
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
			val, err := nodeToAny(n.Content[i+1])
			if err != nil {
				return nil, err
			}
			out[k.Value] = val
		}
		return out, nil
	case yaml.SequenceNode:
		out := make([]any, 0, len(n.Content))
		for _, c := range n.Content {
			val, err := nodeToAny(c)
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
