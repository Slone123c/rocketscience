package core

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type OverrideRule struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
	Policy  string `json:"policy"`
}
type OverrideGroup struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Enabled bool           `json:"enabled"`
	Rules   []OverrideRule `json:"rules"`
}
type SkippedOverride struct {
	GroupID string       `json:"groupId"`
	Rule    OverrideRule `json:"rule"`
	Reason  string       `json:"reason"`
}
type OverrideState struct {
	Groups   []OverrideGroup   `json:"groups"`
	Revision uint64            `json:"revision"`
	Policies []string          `json:"policies"`
	Skipped  []SkippedOverride `json:"skipped"`
	Origins  []string          `json:"origins"`
}

func defaultOverrides() []OverrideGroup {
	return []OverrideGroup{{ID: "default", Name: "我的规则", Enabled: true, Rules: []OverrideRule{}}}
}
func yamlField(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}
func sequence(node *yaml.Node) *yaml.Node {
	if node != nil && node.Kind == yaml.AliasNode {
		return node.Alias
	}
	return node
}
func mergeOverrides(content string, groups []OverrideGroup) (string, []string, []SkippedOverride, []string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return "", nil, nil, nil, err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return "", nil, nil, nil, errors.New("配置必须是 YAML 对象")
	}
	root := doc.Content[0]
	policies := map[string]bool{"DIRECT": true, "REJECT": true}
	var names []string
	for _, field := range []string{"proxy-groups", "proxies"} {
		nodes := sequence(yamlField(root, field))
		if nodes == nil {
			continue
		}
		if nodes.Kind != yaml.SequenceNode {
			return "", nil, nil, nil, fmt.Errorf("%s 必须是列表", field)
		}
		for _, n := range nodes.Content {
			n = sequence(n)
			if name := yamlField(n, "name"); name != nil && name.Value != "" {
				policies[name.Value] = true
				if field == "proxy-groups" {
					names = append(names, name.Value)
				}
			}
		}
	}
	sort.Strings(names)
	names = uniqueStrings(names)
	var prefix []*yaml.Node
	skipped := []SkippedOverride{}
	origins := []string{}
	for _, g := range groups {
		if !g.Enabled {
			continue
		}
		for _, r := range g.Rules {
			if !policies[r.Policy] {
				skipped = append(skipped, SkippedOverride{g.ID, r, "当前配置没有此策略"})
				continue
			}
			prefix = append(prefix, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: r.Type + "," + r.Payload + "," + r.Policy})
			origins = append(origins, g.ID)
		}
	}
	if len(prefix) == 0 {
		return content, names, skipped, origins, nil
	}
	old := sequence(yamlField(root, "rules"))
	if old != nil && old.Kind != yaml.SequenceNode {
		return "", nil, nil, nil, errors.New("rules 必须是列表")
	}
	merged := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: prefix}
	if old != nil {
		merged.Content = append(merged.Content, old.Content...)
	}
	replaced := false
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "rules" {
			root.Content[i+1] = merged
			replaced = true
			break
		}
	}
	if !replaced {
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "rules"}, merged)
	}
	out, err := yaml.Marshal(&doc)
	return string(out), names, skipped, origins, err
}
func uniqueStrings(a []string) []string {
	out := []string{}
	for _, s := range a {
		if len(out) == 0 || out[len(out)-1] != s {
			out = append(out, s)
		}
	}
	return out
}
func validateOverrides(groups []OverrideGroup) error {
	if len(groups) == 0 {
		return errors.New("至少保留一个覆盖规则分组")
	}
	if len(groups) > 100 {
		return errors.New("覆盖分组过多")
	}
	seen := map[string]bool{}
	allowed := map[string]bool{"DOMAIN-SUFFIX": true, "DOMAIN": true, "DOMAIN-KEYWORD": true, "IP-CIDR": true, "IP-CIDR6": true, "PROCESS-NAME": true}
	for _, g := range groups {
		if strings.TrimSpace(g.ID) == "" || strings.TrimSpace(g.Name) == "" || seen[g.ID] {
			return errors.New("分组名称或标识无效")
		}
		seen[g.ID] = true
		if len(g.Rules) > 1000 {
			return errors.New("分组规则过多")
		}
		seenRules := map[string]bool{}
		for _, r := range g.Rules {
			if !allowed[r.Type] || strings.TrimSpace(r.Payload) == "" || strings.TrimSpace(r.Policy) == "" || strings.ContainsAny(r.Payload+r.Policy, ",\r\n") {
				return errors.New("规则类型、内容或走向无效")
			}
			key := r.Type + "\x00" + r.Payload
			if seenRules[key] {
				return errors.New("分组内已有相同匹配规则")
			}
			seenRules[key] = true
		}
	}
	return nil
}

func validateOverrideSyntax(groups []OverrideGroup) error {
	var rules []string
	for _, group := range groups {
		for _, rule := range group.Rules {
			rules = append(rules, rule.Type+","+rule.Payload+",DIRECT")
		}
	}
	if len(rules) == 0 {
		return nil
	}
	data, err := yaml.Marshal(map[string]any{"rules": rules})
	if err != nil {
		return err
	}
	return validateProfile(string(data))
}
func activeContent(d profileData, draft string) string {
	for _, v := range d.Profiles {
		if v.ID == d.ActiveID {
			return v.Content
		}
	}
	return draft
}
func (p *Profiles) Overrides() (OverrideState, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return OverrideState{}, err
	}
	state := OverrideState{Groups: d.Overrides, Revision: d.OverrideRevision, Policies: []string{}, Skipped: []SkippedOverride{}, Origins: []string{}}
	if raw := activeContent(d, p.draft); raw != "" {
		_, state.Policies, state.Skipped, state.Origins, err = mergeOverrides(raw, d.Overrides)
	}
	return state, err
}

// SetOverrides validates and applies a whole revision before persisting it.
func (p *Profiles) SetOverrides(groups []OverrideGroup, expected uint64) (OverrideState, error) {
	if err := validateOverrides(groups); err != nil {
		return OverrideState{}, err
	}
	if err := validateOverrideSyntax(groups); err != nil {
		return OverrideState{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return OverrideState{}, err
	}
	if expected != d.OverrideRevision {
		return OverrideState{}, errors.New("覆盖规则已更改，请刷新后重试")
	}
	raw := activeContent(d, p.draft)
	state := OverrideState{Groups: groups, Revision: expected + 1, Policies: []string{}, Skipped: []SkippedOverride{}, Origins: []string{}}
	var merged string
	if raw != "" {
		merged, state.Policies, state.Skipped, state.Origins, err = mergeOverrides(raw, groups)
		if err != nil {
			return OverrideState{}, err
		}
		if err = validateProfile(merged); err != nil {
			return OverrideState{}, err
		}
	}
	d.Overrides = groups
	d.OverrideRevision++
	persist := func() error { return p.write(d) }
	if raw != "" && p.service != nil {
		err = p.service.commitConfiguration(merged, persist)
	} else {
		err = persist()
	}
	if err != nil {
		return OverrideState{}, err
	}
	return state, nil
}

// StartDraft applies overrides even when a profile has not been saved yet.
func (p *Profiles) StartDraft(content string, port int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.service == nil {
		return errors.New("内核服务不可用")
	}
	d, err := p.load()
	if err != nil {
		return err
	}
	merged, _, _, _, err := mergeOverrides(content, d.Overrides)
	if err != nil {
		return err
	}
	if err = p.service.start(merged, port, d.ProxyBypass, lanValue(d.LAN), coreOptionsValue(d.CoreOptions)); err != nil {
		return err
	}
	p.draft = content
	return nil
}
