package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// TestGroup measures nodes without changing the users selected route.
func (s *Service) TestGroup(group, target string, timeoutMS int) (map[string]int, error) {
	if group == "" {
		return nil, errors.New("请选择策略组")
	}
	u, err := url.Parse(target)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("测速地址必须为 HTTP 或 HTTPS")
	}
	if timeoutMS < 100 || timeoutMS > 30000 {
		return nil, errors.New("测速超时必须为 100 到 30000 毫秒")
	}
	s.mu.Lock()
	if s.cmd == nil {
		s.mu.Unlock()
		return nil, errors.New("内核未启动")
	}
	authority, secret := s.controllerAuthority(), s.secret
	startedAt, content := s.startedAt, s.currentContent
	s.mu.Unlock()
	params := url.Values{"url": {target}, "timeout": {fmt.Sprint(timeoutMS)}}
	endpoint := fmt.Sprintf("http://%s/group/%s/delay?%s", authority, url.PathEscape(group), params.Encode())
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	client := http.Client{Timeout: time.Duration(timeoutMS)*time.Millisecond + 5*time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("测速失败：%s", res.Status)
	}
	result := map[string]int{}
	if err = json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("测速结果无效：%w", err)
	}
	s.mu.Lock()
	unchanged := s.cmd != nil && s.startedAt.Equal(startedAt) && s.currentContent == content
	s.mu.Unlock()
	if !unchanged {
		return nil, errors.New("内核或配置已改变，请重新测速")
	}
	return result, nil
}
