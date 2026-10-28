package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"time"
)

type Proxy struct {
	Name  string      `json:"name"`
	Type  string      `json:"type"`
	Now   string      `json:"now"`
	All   []string    `json:"all"`
	Nodes []ProxyNode `json:"nodes"`
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
	var result struct {
		Proxies map[string]proxyDetail `json:"proxies"`
	}
	if err := s.get("/proxies", &result); err != nil {
		return nil, err
	}
	groups := []Proxy{}
	for name, detail := range result.Proxies {
		v := detail.Proxy
		if v.Name == "" {
			v.Name = name
		}
		switch v.Type {
		case "Selector", "URLTest", "Fallback", "LoadBalance":
			v.Nodes = make([]ProxyNode, 0, len(v.All))
			for _, member := range v.All {
				data := result.Proxies[member]
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
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups, nil
}
func (s *Service) request(method, path string, body any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(method, "http://"+s.controllerAuthority()+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.secret)
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("内核接口返回 %s", res.Status)
	}
	return nil
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
	return s.request("PUT", "/proxies/"+url.PathEscape(group), map[string]string{"name": node})
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
	var v Proxy
	if err := s.get("/proxies/"+url.PathEscape(group), &v); err != nil {
		return err
	}
	if v.Type != "Selector" || v.Now != current {
		return errors.New("策略组已改变，请重新检查")
	}
	for _, candidate := range v.All {
		if candidate == node {
			return s.request("PUT", "/proxies/"+url.PathEscape(group), map[string]string{"name": node})
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
	if err := s.request("PATCH", "/configs", map[string]string{"mode": mode}); err != nil {
		return err
	}
	s.modeRevision++
	return nil
}
