package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Connectivity struct {
	Core            bool                 `json:"core"`
	Request         bool                 `json:"request"`
	Message         string               `json:"message"`
	NodeStatus      string               `json:"nodeStatus"`
	NodeDetail      string               `json:"nodeDetail"`
	Recommendations []NodeRecommendation `json:"recommendations"`
}

// NodeRecommendation is an observation, not a route change.
type NodeRecommendation struct {
	Group   string `json:"group"`
	Current string `json:"current"`
	Node    string `json:"node"`
	Delay   int    `json:"delay"`
}

// CheckConnectivity requests a test URL through the mixed port without changing routes.
func (s *Service) CheckConnectivity(target string) (result Connectivity, resultErr error) {
	u, err := url.Parse(target)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return Connectivity{}, errors.New("检查地址必须为 HTTP 或 HTTPS")
	}
	s.mu.Lock()
	if s.cmd == nil {
		s.mu.Unlock()
		return Connectivity{Message: "内核未启动", NodeStatus: "skip"}, nil
	}
	var v any
	if err = s.get("/version", &v); err != nil {
		s.mu.Unlock()
		return Connectivity{Message: "无法连接内核", NodeStatus: "skip"}, nil
	}
	port := s.mixedPort
	cmd, startedAt, content, modeRevision := s.cmd, s.startedAt, s.currentContent, s.modeRevision
	options, _ := json.Marshal(s.coreOptions)
	s.mu.Unlock()
	var selections map[string]string
	defer func() {
		unchanged := func() bool {
			s.mu.Lock()
			defer s.mu.Unlock()
			currentOptions, _ := json.Marshal(s.coreOptions)
			return s.cmd == cmd && s.startedAt.Equal(startedAt) && s.currentContent == content && s.modeRevision == modeRevision && bytes.Equal(options, currentOptions)
		}
		valid := unchanged()
		if valid && selections != nil {
			latest, err := s.Groups()
			if err != nil {
				valid = false
			} else {
				count := 0
				for _, group := range latest {
					if group.Type != "Selector" || group.Now == "" {
						continue
					}
					count++
					if previous, ok := selections[group.Name]; !ok || previous != group.Now {
						valid = false
					}
				}
				valid = valid && count == len(selections)
			}
		}
		if !valid || !unchanged() {
			result = Connectivity{}
			resultErr = errors.New("检查期间内核、配置或出站选择已改变，请重新检查")
		}
	}()
	result = Connectivity{Core: true, NodeStatus: "skip", NodeDetail: "没有可检查的手动选择策略组", Recommendations: []NodeRecommendation{}}
	groups, groupErr := s.Groups()
	if groupErr != nil {
		result.NodeStatus, result.NodeDetail = "unknown", "无法读取策略组"
	} else {
		selections = make(map[string]string)
		for _, group := range groups {
			if group.Type == "Selector" && group.Now != "" {
				selections[group.Name] = group.Now
			}
		}
		checked, failed, unknown := 0, 0, 0
		type outcome struct {
			delays map[string]int
			err    error
		}
		outcomes := make([]outcome, len(groups))
		sem := make(chan struct{}, 4)
		var wg sync.WaitGroup
		for i, group := range groups {
			if group.Type != "Selector" || group.Now == "" {
				continue
			}
			wg.Add(1)
			go func(i int, group Proxy) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				outcomes[i].delays, outcomes[i].err = s.TestGroup(group.Name, target, 5000)
			}(i, group)
		}
		wg.Wait()
		for i, group := range groups {
			if group.Type != "Selector" || group.Now == "" {
				continue
			}
			checked++
			delays, testErr := outcomes[i].delays, outcomes[i].err
			if testErr != nil {
				unknown++
				continue
			}
			if delay, ok := delays[group.Now]; !ok || delay <= 0 {
				failed++
				var best string
				bestDelay := 0
				for _, name := range group.All {
					switch strings.ToUpper(name) {
					case "DIRECT", "REJECT", "REJECT-DROP", "PASS":
						continue
					}
					delay := delays[name]
					if name != group.Now && delay > 0 && (bestDelay == 0 || delay < bestDelay) {
						best, bestDelay = name, delay
					}
				}
				if best != "" {
					result.Recommendations = append(result.Recommendations, NodeRecommendation{Group: group.Name, Current: group.Now, Node: best, Delay: bestDelay})
				}
			}
		}
		switch {
		case checked == 0:
		case failed > 0:
			result.NodeStatus, result.NodeDetail = "fail", fmt.Sprintf("%d 个已选节点未响应", failed)
		case unknown > 0:
			result.NodeStatus, result.NodeDetail = "unknown", "部分策略组无法测速"
		default:
			result.NodeStatus, result.NodeDetail = "ok", fmt.Sprintf("%d 个策略组的已选节点有响应", checked)
		}
	}
	proxy, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
	transport := &http.Transport{Proxy: http.ProxyURL(proxy), DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := http.Client{Transport: transport, Timeout: 8 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Get(target)
	if err != nil {
		result.Message = "测试请求失败，请检查节点、网络或测试地址"
		return result, nil
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		result.Message = fmt.Sprintf("测试地址返回 HTTP %d", res.StatusCode)
		return result, nil
	}
	result.Request, result.Message = true, "测试请求通过"
	return result, nil
}
