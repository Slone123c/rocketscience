package core

import (
	"context"
	"errors"
	"net/url"
)

// TestGroup measures nodes without changing the users selected route.
func (s *Service) TestGroup(group, target string, timeoutMS int) (map[string]int, error) {
	if err := validateDelayTest(group, target, timeoutMS); err != nil {
		return nil, err
	}
	s.mu.Lock()
	if s.cmd == nil {
		s.mu.Unlock()
		return nil, errors.New("内核未启动")
	}
	startedAt, content := s.startedAt, s.currentContent
	s.mu.Unlock()
	result, err := testInProcessGroupDelay(context.Background(), group, target, timeoutMS)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	unchanged := s.cmd != nil && s.startedAt.Equal(startedAt) && s.currentContent == content
	s.mu.Unlock()
	if !unchanged {
		return nil, errors.New("内核或配置已改变，请重新测速")
	}
	return result, nil
}

func validateDelayTest(group, target string, timeoutMS int) error {
	if group == "" {
		return errors.New("请选择策略组")
	}
	u, err := url.Parse(target)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("测速地址必须为 HTTP 或 HTTPS")
	}
	if timeoutMS < 100 || timeoutMS > 30000 {
		return errors.New("测速超时必须为 100 到 30000 毫秒")
	}
	return nil
}
