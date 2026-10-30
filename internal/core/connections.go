package core

import "errors"

type ConnectionMetadata struct {
	Host            string `json:"host"`
	DestinationIP   string `json:"destinationIP"`
	DestinationPort string `json:"destinationPort"`
	Network         string `json:"network"`
	Process         string `json:"process"`
	ProcessPath     string `json:"processPath"`
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
	if s.cmd == nil {
		return ConnectionSnapshot{Connections: []Connection{}}, nil
	}
	return inProcessConnectionsSnapshot(), nil
}
func (s *Service) Rules() ([]Rule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil {
		return []Rule{}, nil
	}
	return inProcessRules(), nil
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
	return closeInProcessConnection(id)
}

func (s *Service) CloseAllConnections() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil {
		return errors.New("内核未启动")
	}
	closeAllInProcessConnections()
	return nil
}
