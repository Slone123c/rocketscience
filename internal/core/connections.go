package core

import (
	"errors"
	"net/url"
)

type ConnectionMetadata struct {
	Host            string `json:"host"`
	DestinationIP   string `json:"destinationIP"`
	DestinationPort string `json:"destinationPort"`
	Network         string `json:"network"`
	Process         string `json:"process"`
}
type Connection struct {
	ID          string             `json:"id"`
	Metadata    ConnectionMetadata `json:"metadata"`
	Upload      int64              `json:"upload"`
	Download    int64              `json:"download"`
	Start       string             `json:"start"`
	Chains      []string           `json:"chains"`
	Rule        string             `json:"rule"`
	RulePayload string             `json:"rulePayload"`
}
type ConnectionSnapshot struct {
	Connections   []Connection `json:"connections"`
	UploadTotal   int64        `json:"uploadTotal"`
	DownloadTotal int64        `json:"downloadTotal"`
}
type Rule struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
	Proxy   string `json:"proxy"`
}

func (s *Service) Connections() (ConnectionSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := ConnectionSnapshot{Connections: []Connection{}}
	if s.cmd == nil {
		return result, nil
	}
	err := s.get("/connections", &result)
	if result.Connections == nil {
		result.Connections = []Connection{}
	}
	return result, err
}
func (s *Service) Rules() ([]Rule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil {
		return []Rule{}, nil
	}
	var result struct {
		Rules []Rule `json:"rules"`
	}
	if err := s.get("/rules", &result); err != nil {
		return nil, err
	}
	if result.Rules == nil {
		return []Rule{}, nil
	}
	return result.Rules, nil
}
func (s *Service) CloseConnection(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil {
		return errors.New("内核未启动")
	}
	if id == "" {
		return errors.New("连接不存在")
	}
	return s.request("DELETE", "/connections/"+url.PathEscape(id), nil)
}

func (s *Service) CloseAllConnections() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil {
		return errors.New("内核未启动")
	}
	return s.request("DELETE", "/connections", nil)
}
