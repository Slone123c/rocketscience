package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"gopkg.in/yaml.v3"
)

type geoSession struct {
	Version    int    `json:"version"`
	Runtime    string `json:"runtime"`
	PID        int    `json:"pid"`
	Dirty      bool   `json:"dirty"`
	AutoUpdate bool   `json:"autoUpdate"`
}

func (c *geoCache) journalPath(runtime string) (string, error) {
	base, err := c.path()
	if err != nil {
		return "", err
	}
	name := filepath.Base(runtime)
	if !strings.HasPrefix(name, "rocketscience-core-") {
		return "", errors.New("无效的内核目录")
	}
	return filepath.Join(base, "sessions", name+".json"), nil
}
func validGeoRuntime(runtime string) error {
	original, err := os.Lstat(runtime)
	if err != nil {
		return err
	}
	if !original.IsDir() || original.Mode()&os.ModeSymlink != 0 {
		return errors.New("GEO 临时目录不能为符号链接")
	}
	root, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return err
	}
	actual, err := filepath.EvalSymlinks(runtime)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, actual)
	if err != nil || filepath.Dir(rel) != "." || !strings.HasPrefix(filepath.Base(rel), "rocketscience-core-") {
		return errors.New("GEO 临时目录不在预期位置")
	}
	info, err := os.Lstat(actual)
	if err != nil {
		return err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(st.Uid) != os.Getuid() || !info.IsDir() || info.Mode().Perm() != 0700 {
		return errors.New("GEO 临时目录权限无效")
	}
	return nil
}
func (c *geoCache) writeSession(session geoSession) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	path, err := c.journalPath(session.Runtime)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".session-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
func (c *geoCache) removeSession(runtime string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	path, err := c.journalPath(runtime)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
func processAlive(pid int) bool {
	if pid <= 1 {
		return true
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// Once automatic updates have been enabled during a run, recovery must treat
// an unexpected app exit as potentially interrupting an update.
func (s *Service) markGeoAuto(config []byte) error {
	var state struct {
		Auto bool `yaml:"geo-auto-update"`
	}
	if err := yaml.Unmarshal(config, &state); err != nil {
		return err
	}
	if !state.Auto || s.geoSession.AutoUpdate {
		return nil
	}
	session := s.geoSession
	session.AutoUpdate = true
	if err := s.geo.writeSession(session); err != nil {
		return fmt.Errorf("无法记录 GEO 自动更新状态：%w", err)
	}
	s.geoSession = session
	return nil
}

// recover only promotes data when the recorded process is gone and no update
// could have been interrupted. Ambiguous sessions remain untouched for review.
func (c *geoCache) recover(exclude string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	base, err := c.path()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(base, "sessions"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	warnings := []string{}
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasPrefix(entry.Name(), "rocketscience-core-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(base, "sessions", entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			warnings = append(warnings, "无法读取恢复记录 "+entry.Name())
			continue
		}
		var session geoSession
		if json.Unmarshal(data, &session) != nil || session.Version != 1 || filepath.Base(session.Runtime)+".json" != entry.Name() {
			warnings = append(warnings, "GEO 恢复记录无效："+entry.Name())
			continue
		}
		if session.Runtime == exclude {
			continue
		}
		if _, err := os.Lstat(session.Runtime); errors.Is(err, os.ErrNotExist) {
			_ = os.Remove(path)
			continue
		}
		if err := validGeoRuntime(session.Runtime); err != nil {
			warnings = append(warnings, "GEO 临时目录需要检查："+entry.Name())
			continue
		}
		if processAlive(session.PID) || session.Dirty || session.AutoUpdate {
			warnings = append(warnings, "GEO 数据待确认，已保留："+session.Runtime)
			continue
		}
		if err := copyGeoFiles(session.Runtime, base); err != nil {
			warnings = append(warnings, fmt.Sprintf("GEO 恢复失败（%s）：%v", session.Runtime, err))
			continue
		}
		if err := os.Remove(path); err != nil {
			warnings = append(warnings, "无法清除 GEO 恢复记录："+entry.Name())
			continue
		}
		if err := os.RemoveAll(session.Runtime); err != nil {
			warnings = append(warnings, "无法清理 GEO 临时目录："+session.Runtime)
		}
	}
	return warnings, nil
}
