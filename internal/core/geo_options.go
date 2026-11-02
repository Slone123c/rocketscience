package core

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type GeoOptions struct {
	DataMode       *bool `json:"dataMode"`
	AutoUpdate     *bool `json:"autoUpdate"`
	UpdateInterval *int  `json:"updateInterval"`
}

func validateGeoOptions(v GeoOptions) error {
	if v.UpdateInterval != nil && (*v.UpdateInterval < 1 || *v.UpdateInterval > 8760) {
		return errors.New("GEO 更新间隔必须为 1 到 8760 小时")
	}
	return nil
}
func (p *Profiles) GeoSettings() (GeoOptions, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return GeoOptions{}, err
	}
	options := coreOptionsValue(d.CoreOptions)
	if options.Geo == nil {
		return GeoOptions{}, nil
	}
	return *options.Geo, nil
}
func (p *Profiles) SetGeoSettings(value GeoOptions) (GeoOptions, error) {
	if err := validateGeoOptions(value); err != nil {
		return GeoOptions{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return GeoOptions{}, err
	}
	options := coreOptionsValue(d.CoreOptions)
	options.Geo = &value
	if _, err = p.setCoreOptionsLocked(d, options); err != nil {
		return GeoOptions{}, err
	}
	return value, nil
}

// UpdateGeo waits for the core's update to finish and saves its completed files.
func (s *Service) UpdateGeo() error {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	s.mu.Lock()
	if s.cmd == nil {
		s.mu.Unlock()
		return errors.New("请先启动内核")
	}
	dir, session, updateFn := s.runtimeDir, s.geoSession, s.updateGeo
	s.mu.Unlock()
	if updateFn == nil {
		updateFn = updateInProcessGeoDatabases
	}
	session.Dirty = true
	if err := s.geo.writeSession(session); err != nil {
		return fmt.Errorf("无法记录 GEO 更新，未开始下载：%w", err)
	}
	s.mu.Lock()
	s.geoSession = session
	s.geoDirtyFailed = true
	s.mu.Unlock()
	if err := updateFn(); err != nil {
		s.mu.Lock()
		s.geoDirtyFailed = true
		s.mu.Unlock()
		return fmt.Errorf("GEO 更新失败：%w", err)
	}
	// A TUN supervisor may need one heartbeat to return file ownership.
	var saveErr error
	for attempt := 0; attempt < 4; attempt++ {
		saveErr = s.geo.save(dir)
		if saveErr == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if saveErr != nil {
		s.mu.Lock()
		s.geoDirtyFailed = true
		s.geoWarning = "GEO 更新完成但保存失败，临时文件位于 " + dir
		s.mu.Unlock()
		return saveErr
	}
	session.Dirty = false
	if err := s.geo.writeSession(session); err != nil {
		return fmt.Errorf("GEO 已下载，但恢复记录未更新：%w", err)
	}
	warnings, recoveryErr := s.geo.recover(dir)
	s.mu.Lock()
	s.geoSession = session
	s.geoDirtyFailed = false
	if recoveryErr != nil {
		s.geoWarning = "无法检查 GEO 恢复记录：" + recoveryErr.Error()
	} else {
		s.geoWarning = strings.Join(warnings, "；")
	}
	s.mu.Unlock()
	return nil
}

func (s *Service) RecoverGeoCache() error {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	s.mu.Lock()
	dir := s.runtimeDir
	if s.cmd == nil {
		dir = ""
	}
	s.mu.Unlock()
	warnings, err := s.geo.recover(dir)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.geoWarning = "无法检查 GEO 恢复记录：" + err.Error()
		return err
	}
	s.geoWarning = strings.Join(warnings, "；")
	if len(warnings) > 0 {
		return errors.New(s.geoWarning)
	}
	return nil
}

// InstallBundledGeo copies databases shipped in the app into the GEO cache
// when they are not there yet.
func (s *Service) InstallBundledGeo(resourcesDir string) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	return s.geo.installBundled(resourcesDir)
}
