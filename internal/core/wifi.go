package core

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode"

	"rocketscience/internal/network"
)

type WiFiSettings struct {
	Enabled bool     `json:"enabled"`
	Trusted []string `json:"trusted"`
}
type WiFiStatus struct {
	Known          bool   `json:"known"`
	SSID           string `json:"ssid"`
	Detail         string `json:"detail"`
	Enabled        bool   `json:"enabled"`
	Trusted        bool   `json:"trusted"`
	Applied        bool   `json:"applied"`
	ManualOverride bool   `json:"manualOverride"`
}
type wifiModeState struct {
	running  bool
	mode     string
	revision uint64
	runID    time.Time
}
type wifiModeTarget interface {
	wifiMode() (wifiModeState, error)
	wifiSetMode(wifiModeState, string) (wifiModeState, error)
}

func (s *Service) wifiMode() (wifiModeState, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	state := wifiModeState{running: s.cmd != nil, revision: s.modeRevision, runID: s.startedAt}
	if !state.running {
		return state, nil
	}
	state.mode = currentInProcessMode()
	return state, nil
}
func (s *Service) wifiSetMode(expected wifiModeState, mode string) (wifiModeState, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil || s.modeRevision != expected.revision {
		return wifiModeState{}, errors.New("内核或手动选择已改变，未自动切换模式")
	}
	currentMode := currentInProcessMode()
	if currentMode != expected.mode {
		return wifiModeState{}, errors.New("出站模式已改变，未覆盖当前选择")
	}
	if err := setInProcessMode(mode); err != nil {
		return wifiModeState{}, err
	}
	if !strings.EqualFold(currentMode, mode) {
		closeAllInProcessConnections()
	}
	return wifiModeState{running: true, mode: mode, revision: s.modeRevision, runID: s.startedAt}, nil
}
func (p *Profiles) WiFiSettings() (WiFiSettings, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return WiFiSettings{}, err
	}
	if d.WiFi == nil {
		return WiFiSettings{Trusted: []string{}}, nil
	}
	value := *d.WiFi
	value.Trusted = append([]string{}, value.Trusted...)
	return value, nil
}
func (p *Profiles) SetWiFiSettings(value WiFiSettings) (WiFiSettings, error) {
	if len(value.Trusted) > 100 {
		return WiFiSettings{}, errors.New("最多保存 100 个可信 Wi-Fi 名称")
	}
	seen := map[string]bool{}
	entries := []string{}
	for _, name := range value.Trusted {
		if name == "" || len([]byte(name)) > 32 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
			return WiFiSettings{}, errors.New("Wi-Fi 名称不能为空、含控制字符或超过 32 字节")
		}
		if !seen[name] {
			seen[name] = true
			entries = append(entries, name)
		}
	}
	value.Trusted = entries
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return WiFiSettings{}, err
	}
	d.WiFi = &value
	if err = p.write(d); err != nil {
		return WiFiSettings{}, err
	}
	return value, nil
}

type WiFi struct {
	profiles   *Profiles
	target     wifiModeTarget
	read       func() (network.Snapshot, error)
	tickMu     sync.Mutex
	mu         sync.Mutex
	status     WiFiStatus
	key        string
	suppressed bool
	previous   string
	owned      *wifiModeState
	last       wifiModeState
}

func NewWiFi(profiles *Profiles, service *Service, read func() (network.Snapshot, error)) *WiFi {
	return &WiFi{profiles: profiles, target: service, read: read}
}
func (w *WiFi) State() WiFiStatus               { w.mu.Lock(); defer w.mu.Unlock(); return w.status }
func (w *WiFi) Settings() (WiFiSettings, error) { return w.profiles.WiFiSettings() }
func (w *WiFi) SetSettings(value WiFiSettings) (WiFiSettings, error) {
	saved, err := w.profiles.SetWiFiSettings(value)
	if err != nil {
		return WiFiSettings{}, err
	}
	if err = w.Check(); err != nil {
		return saved, err
	}
	return saved, nil
}
func (w *WiFi) Check() error {
	w.tickMu.Lock()
	defer w.tickMu.Unlock()
	config, err := w.profiles.WiFiSettings()
	if err != nil {
		return err
	}
	snapshot := network.Snapshot{Detail: "可信 Wi-Fi 直连未启用"}
	if config.Enabled {
		if w.read == nil {
			snapshot.Detail = "无法读取 Wi-Fi 状态"
		} else {
			snapshot, err = w.read()
			if err != nil {
				snapshot = network.Snapshot{Detail: err.Error()}
			}
		}
	}
	trusted := false
	for _, name := range config.Trusted {
		if snapshot.Known && snapshot.SSID != "" && snapshot.SSID == name {
			trusted = true
		}
	}
	trusted = trusted && config.Enabled
	encoded, _ := json.Marshal(config)
	key := string(encoded) + "\x00" + snapshot.SSID
	if snapshot.Known {
		key += "\x00known"
	}
	networkChanged := key != w.key
	if networkChanged {
		w.suppressed = false
		w.key = key
	}
	if !config.Enabled && w.owned == nil {
		w.last = wifiModeState{}
		w.mu.Lock()
		w.status = WiFiStatus{Detail: snapshot.Detail}
		w.mu.Unlock()
		return nil
	}
	current, modeErr := w.target.wifiMode()
	state := WiFiStatus{Known: snapshot.Known, SSID: snapshot.SSID, Detail: snapshot.Detail, Enabled: config.Enabled, Trusted: trusted}
	defer func() {
		state.Applied = w.owned != nil
		state.ManualOverride = w.suppressed
		w.mu.Lock()
		w.status = state
		w.mu.Unlock()
	}()
	if modeErr != nil {
		state.Detail = modeErr.Error()
		return modeErr
	}
	if !current.running {
		w.owned = nil
		w.last = current
		return nil
	}
	restarted := w.last.running && !w.last.runID.Equal(current.runID) && w.last.revision != current.revision
	if restarted {
		w.suppressed = false
		w.owned = nil
	}
	if w.last.running && w.last.revision != current.revision && !networkChanged && !restarted {
		w.suppressed = true
	}
	w.last = current
	if w.owned != nil && (w.owned.revision != current.revision || current.mode != "direct") {
		w.owned = nil
		w.suppressed = true
	}
	if !trusted {
		if w.owned != nil {
			restored, err := w.target.wifiSetMode(current, w.previous)
			if err != nil {
				state.Detail = err.Error()
				return err
			}
			w.last = restored
			w.owned = nil
		}
		return nil
	}
	if w.owned == nil && !w.suppressed && current.mode != "direct" {
		changed, err := w.target.wifiSetMode(current, "direct")
		if err != nil {
			state.Detail = err.Error()
			return err
		}
		w.previous = current.mode
		w.owned = &changed
		w.last = changed
	}
	if w.suppressed {
		state.Detail = "已保留你的手动模式选择，网络改变后再检查"
	} else if w.owned != nil {
		state.Detail = "已在可信 Wi-Fi 上切换为直连"
	}
	return nil
}
func RunWiFi(w *WiFi, stop <-chan struct{}) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			_ = w.Check()
		}
	}
}
