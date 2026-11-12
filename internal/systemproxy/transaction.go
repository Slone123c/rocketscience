// Package systemproxy tracks the fields RocketScience changes in a proxy takeover.
package systemproxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

type Settings map[string]json.RawMessage
type Service struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Settings Settings `json:"settings"`
}
type Change struct {
	ID        string   `json:"id"`
	Expected  Settings `json:"expected"`
	Desired   Settings `json:"desired"`
	Keys      []string `json:"keys"`
	GuardKeys []string `json:"guardKeys,omitempty"`
}

// Backend must compare and apply changes in one locked preferences transaction.
// Conflicting services are not changed; unrelated fields must be preserved.
type Backend interface {
	Snapshot() ([]Service, error)
	CompareAndSwap([]Change) ([]string, error)
}
type journal struct {
	Version int      `json:"version"`
	Changes []Change `json:"changes"`
}
type Manager struct {
	mu      sync.Mutex
	backend Backend
	path    string
}

var ErrAlreadyPending = errors.New("存在未恢复的系统代理记录")
var ErrRecoveryNeeded = errors.New("系统代理操作需要恢复")

func New(backend Backend, path string) *Manager { return &Manager{backend: backend, path: path} }

var keys = []string{"HTTPEnable", "HTTPProxy", "HTTPPort", "HTTPSEnable", "HTTPSProxy", "HTTPSPort", "SOCKSEnable", "SOCKSProxy", "SOCKSPort", "ProxyAutoConfigEnable", "ProxyAutoDiscoveryEnable", "ExceptionsList", "__ProtocolEnabled"}
var keyGroups = [][]string{{"HTTPEnable", "HTTPProxy", "HTTPPort"}, {"HTTPSEnable", "HTTPSProxy", "HTTPSPort"}, {"SOCKSEnable", "SOCKSProxy", "SOCKSPort"}, {"ProxyAutoConfigEnable"}, {"ProxyAutoDiscoveryEnable"}}
var defaultExceptions = []string{"127.0.0.1", "localhost", "*.local", "169.254/16"}

func effectiveExceptions(entries []string) []string {
	out := make([]string, 0, len(defaultExceptions)+len(entries))
	seen := make(map[string]bool, len(defaultExceptions)+len(entries))
	for _, item := range defaultExceptions {
		seen[item] = true
		out = append(out, item)
	}
	for _, raw := range entries {
		key := raw
		for i := 0; i < len(key); i++ {
			if key[i] >= 'A' && key[i] <= 'Z' {
				b := []byte(key)
				for j := i; j < len(b); j++ {
					if b[j] >= 'A' && b[j] <= 'Z' {
						b[j] += 'a' - 'A'
					}
				}
				key = string(b)
				break
			}
		}
		if key != "" && !seen[key] {
			seen[key] = true
			out = append(out, raw)
		}
	}
	return out
}

func takeover(port int) Settings {
	return Settings{"HTTPEnable": json.RawMessage("1"), "HTTPProxy": json.RawMessage(`"127.0.0.1"`), "HTTPPort": json.RawMessage(strconv.Itoa(port)), "HTTPSEnable": json.RawMessage("1"), "HTTPSProxy": json.RawMessage(`"127.0.0.1"`), "HTTPSPort": json.RawMessage(strconv.Itoa(port)), "SOCKSEnable": json.RawMessage("1"), "SOCKSProxy": json.RawMessage(`"127.0.0.1"`), "SOCKSPort": json.RawMessage(strconv.Itoa(port)), "ProxyAutoConfigEnable": json.RawMessage("0"), "ProxyAutoDiscoveryEnable": json.RawMessage("0"), "__ProtocolEnabled": json.RawMessage("true")}
}
func pickKeys(settings Settings, selected []string) Settings {
	result := Settings{}
	for _, key := range selected {
		if v, ok := settings[key]; ok {
			result[key] = append(json.RawMessage(nil), v...)
		}
	}
	return result
}
func pick(settings Settings) Settings { return pickKeys(settings, keys) }

// Apply persists a recovery record before changing preferences.
func (m *Manager) Apply(port int) error {
	return m.ApplyWithBypass(port, nil)
}

// ApplyWithBypass leaves the existing exceptions untouched when bypass is nil.
func (m *Manager) ApplyWithBypass(port int, bypass *[]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if port < 1 || port > 65535 {
		return errors.New("端口无效")
	}
	if _, err := os.Stat(m.path); err == nil {
		return ErrAlreadyPending
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	services, err := m.backend.Snapshot()
	if err != nil {
		return err
	}
	if len(services) == 0 {
		return errors.New("没有可用的网络服务")
	}
	j := journal{Version: 1}
	for _, s := range services {
		if s.ID == "" {
			continue
		}
		var enabled bool
		if value, ok := s.Settings["__ProtocolEnabled"]; !ok || json.Unmarshal(value, &enabled) != nil {
			continue
		}
		desired := takeover(port)
		groups := append([][]string(nil), keyGroups...)
		if bypass != nil {
			value, err := json.Marshal(effectiveExceptions(*bypass))
			if err != nil {
				return err
			}
			desired["ExceptionsList"] = value
			groups = append(groups, []string{"ExceptionsList"})
		}
		guardKeys := append([]string(nil), keys...)
		if bypass == nil {
			guardKeys = guardKeys[:len(guardKeys)-2]
			guardKeys = append(guardKeys, "__ProtocolEnabled")
		}
		for _, group := range groups {
			j.Changes = append(j.Changes, Change{ID: s.ID, Expected: pickKeys(s.Settings, group), Desired: pickKeys(desired, group), Keys: append([]string(nil), group...)})
		}
		j.Changes = append(j.Changes, Change{ID: s.ID, Expected: pickKeys(s.Settings, guardKeys), Desired: desired, Keys: []string{"__ProtocolEnabled"}, GuardKeys: guardKeys})
		if native, ok := platformNativeJournalChange(s, port); ok {
			j.Changes = append(j.Changes, native)
		}
	}
	if len(j.Changes) == 0 {
		return errors.New("没有支持代理协议的网络服务")
	}
	if err = m.write(j); err != nil {
		return err
	}
	conflicts, err := m.backend.CompareAndSwap(j.Changes)
	if err != nil {
		return errors.Join(ErrRecoveryNeeded, fmt.Errorf("设置系统代理失败，保留恢复记录：%w", err))
	}
	if len(conflicts) > 0 {
		return errors.Join(ErrRecoveryNeeded, fmt.Errorf("系统代理已被其他程序修改，保留恢复记录：%v", conflicts))
	}
	return nil
}

// Restore changes only services whose relevant settings are still ours.
func (m *Manager) Restore() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, err := m.read()
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	changes := make([]Change, 0, len(j.Changes))
	for _, c := range j.Changes {
		changes = append(changes, Change{ID: c.ID, Expected: c.Desired, Desired: c.Expected, Keys: c.Keys, GuardKeys: c.GuardKeys})
	}
	conflicts, err := m.backend.CompareAndSwap(changes)
	if err != nil {
		return nil, fmt.Errorf("恢复系统代理失败，保留恢复记录：%w", err)
	}
	if err = os.Remove(m.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return conflicts, err
	}
	return conflicts, nil
}
func (m *Manager) Pending() (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, err := m.read()
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}
func (m *Manager) read() (journal, error) {
	var j journal
	b, err := os.ReadFile(m.path)
	if err != nil {
		return j, err
	}
	if err = json.Unmarshal(b, &j); err != nil {
		return j, fmt.Errorf("恢复记录损坏：%w", err)
	}
	if j.Version != 1 || len(j.Changes) == 0 {
		return j, errors.New("恢复记录无效")
	}
	if err := validateJournalChanges(j.Changes); err != nil {
		return j, err
	}
	return j, nil
}
func (m *Manager) write(j journal) error {
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	b, err := json.Marshal(j)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".proxy-recovery-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	if e := f.Close(); err == nil {
		err = e
	}
	if err != nil {
		return err
	}
	if err := os.Link(name, m.path); errors.Is(err, os.ErrExist) {
		return ErrAlreadyPending
	} else if err != nil {
		return err
	}
	if directory, err := os.Open(dir); err == nil {
		defer directory.Close()
		if err = directory.Sync(); err != nil {
			return errors.Join(ErrRecoveryNeeded, err)
		}
	}
	return nil
}
