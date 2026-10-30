package core

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type Proxy struct {
	Name  string      `json:"name"`
	Type  string      `json:"type"`
	Now   string      `json:"now"`
	All   []string    `json:"all"`
	Nodes []ProxyNode `json:"nodes"`
	// Rules counts the rules that target this group; Fallback marks the MATCH target.
	Rules    int  `json:"rules"`
	Fallback bool `json:"fallback"`
}

type ProxyNode struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	UDP     *bool  `json:"udp"`
	Delay   *int   `json:"delay"`
	DelayAt string `json:"delayAt"`
}

type proxyDetail struct {
	Proxy
	UDP     *bool `json:"udp"`
	History []struct {
		Time  time.Time `json:"time"`
		Delay int       `json:"delay"`
	} `json:"history"`
}

func (s *Service) Groups() ([]Proxy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil {
		return []Proxy{}, nil
	}
	proxies, err := inProcessProxiesMap()
	if err != nil {
		return nil, err
	}
	return buildProxyGroups(proxies, inProcessRules()), nil
}

func buildProxyGroups(proxies map[string]proxyDetail, rules []Rule) []Proxy {
	groups := []Proxy{}
	for name, detail := range proxies {
		v := detail.Proxy
		if v.Name == "" {
			v.Name = name
		}
		switch v.Type {
		case "Selector", "URLTest", "Fallback", "LoadBalance":
			v.Nodes = make([]ProxyNode, 0, len(v.All))
			for _, member := range v.All {
				data := proxies[member]
				node := ProxyNode{Name: member, Type: data.Type, UDP: data.UDP}
				if len(data.History) > 0 {
					last := data.History[len(data.History)-1]
					if last.Delay >= 0 {
						delay := last.Delay
						node.Delay = &delay
					}
					if !last.Time.IsZero() {
						node.DelayAt = last.Time.Format(time.RFC3339Nano)
					}
				}
				v.Nodes = append(v.Nodes, node)
			}
			groups = append(groups, v)
		}
	}
	// GLOBAL lists every group in configuration order; keep that order so the
	// subscription's main group stays first. GLOBAL itself goes last.
	order := map[string]int{}
	for i, name := range proxies["GLOBAL"].All {
		order[name] = i + 1
	}
	rank := func(name string) int {
		if name == "GLOBAL" {
			return len(order) + 2
		}
		if i, ok := order[name]; ok {
			return i
		}
		return len(order) + 1
	}
	sort.SliceStable(groups, func(i, j int) bool {
		a, b := rank(groups[i].Name), rank(groups[j].Name)
		if a != b {
			return a < b
		}
		return groups[i].Name < groups[j].Name
	})
	if len(rules) > 0 {
		annotateRuleTargets(groups, rules)
	}
	return groups
}

// annotateRuleTargets records how many rules point at each group and which
// group receives the MATCH fallback.
func annotateRuleTargets(groups []Proxy, rules []Rule) {
	counts := map[string]int{}
	fallback := ""
	for _, rule := range rules {
		counts[rule.Proxy]++
		if strings.EqualFold(rule.Type, "MATCH") {
			fallback = rule.Proxy
		}
	}
	for i := range groups {
		groups[i].Rules = counts[groups[i].Name]
		groups[i].Fallback = fallback != "" && groups[i].Name == fallback
	}
}

// configRules turns `TYPE,payload,target[,options]` lines into rules. Logical
// rules keep commas inside parentheses, so the target is the last field after
// trailing options are dropped.
func configRules(lines []string) []Rule {
	rules := make([]Rule, 0, len(lines))
	for _, line := range lines {
		parts := strings.Split(line, ",")
		for len(parts) > 2 {
			last := strings.TrimSpace(parts[len(parts)-1])
			if last != "no-resolve" && last != "src" {
				break
			}
			parts = parts[:len(parts)-1]
		}
		if len(parts) < 2 {
			continue
		}
		rules = append(rules, Rule{Type: strings.TrimSpace(parts[0]), Proxy: strings.TrimSpace(parts[len(parts)-1])})
	}
	return rules
}

func (s *Service) closeGroupConnectionsLocked(group string) {
	closeGroupInProcessConnections(group)
}

func (s *Service) Select(group, node string) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil {
		return errors.New("内核未启动")
	}
	if group == "" || node == "" {
		return errors.New("请选择策略组与节点")
	}
	proxies, err := inProcessProxiesMap()
	before, hadBefore := proxies[group]
	if err != nil {
		hadBefore = false
	}
	if err := selectInProcessProxy(group, node); err != nil {
		return err
	}
	if !hadBefore || before.Now != node {
		s.closeGroupConnectionsLocked(group)
	}
	s.rememberSelectionLocked(group, node)
	return nil
}

// SelectRecommendation refuses a stale recommendation if the user has changed the group.
func (s *Service) SelectRecommendation(group, current, node string) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil {
		return errors.New("内核未启动")
	}
	proxies, err := inProcessProxiesMap()
	if err != nil {
		return err
	}
	v, ok := proxies[group]
	if !ok {
		return fmt.Errorf("策略组 %s 不存在", group)
	}
	if v.Type != "Selector" || v.Now != current {
		return errors.New("策略组已改变，请重新检查")
	}
	for _, candidate := range v.All {
		if candidate == node {
			if err := selectInProcessProxy(group, node); err != nil {
				return err
			}
			if current != node {
				s.closeGroupConnectionsLocked(group)
			}
			s.rememberSelectionLocked(group, node)
			return nil
		}
	}
	return errors.New("推荐节点已不存在，请重新检查")
}
func (s *Service) SetMode(mode string) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil {
		return errors.New("内核未启动")
	}
	if mode != "rule" && mode != "global" && mode != "direct" {
		return errors.New("无效的出站模式")
	}
	beforeMode := currentInProcessMode()
	if err := setInProcessMode(mode); err != nil {
		return err
	}
	s.modeRevision++
	if !strings.EqualFold(beforeMode, mode) {
		closeAllInProcessConnections()
	}
	return nil
}
