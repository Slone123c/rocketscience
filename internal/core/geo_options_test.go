package core

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestGeoSettingsMergePersistenceAndValidation(t *testing.T) {
	p := NewProfiles(t.TempDir())
	geo := GeoOptions{DataMode: ptr(true), AutoUpdate: ptr(true), UpdateInterval: ptr(12)}
	if _, err := p.SetDNSSettings(DNSOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.SetGeoSettings(geo); err != nil {
		t.Fatal(err)
	}
	if _, err := p.SetCoreSettings(CoreOptions{LogLevel: ptr("warning")}); err != nil {
		t.Fatal(err)
	}
	got, err := NewProfiles(p.dir).GeoSettings()
	if err != nil || got.UpdateInterval == nil || *got.UpdateInterval != 12 {
		t.Fatalf("persist: %+v %v", got, err)
	}
	for _, n := range []int{0, 8761} {
		if _, err := p.SetGeoSettings(GeoOptions{UpdateInterval: &n}); err == nil {
			t.Fatalf("invalid interval %d", n)
		}
	}
	opts, err := p.CoreSettings()
	if err != nil {
		t.Fatal(err)
	}
	b, err := runtimeConfigWithOptions("geo-auto-update: false\nrules: [MATCH,DIRECT]\n", 7890, 9999, "secret", "", LANSettings{}, false, opts)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err = yaml.Unmarshal(b, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg["geodata-mode"] != true || cfg["geo-auto-update"] != true || cfg["geo-update-interval"] != 12 || cfg["log-level"] != "warning" {
		t.Fatalf("merge: %+v", cfg)
	}
}
func TestGeoCacheCopiesOnlyDataAndPreservesOnFailure(t *testing.T) {
	runtime := t.TempDir()
	cache := geoCache{dir: t.TempDir()}
	for name, data := range map[string]string{"GeoIP.dat": "old", "config.yaml": "secret", "geosite.dat": "site"} {
		if err := os.WriteFile(filepath.Join(runtime, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := cache.save(runtime); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cache.dir, "config.yaml")); !os.IsNotExist(err) {
		t.Fatal("cached config")
	}
	restored := t.TempDir()
	if err := cache.seed(restored); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(restored, "GeoIP.dat"))
	if err != nil || string(b) != "old" {
		t.Fatalf("seed %s %v", b, err)
	}
	if err = os.Remove(filepath.Join(runtime, "GeoIP.dat")); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(filepath.Join(runtime, "config.yaml"), filepath.Join(runtime, "GeoIP.dat")); err != nil {
		t.Fatal(err)
	}
	if err = cache.save(runtime); err != nil {
		t.Fatal(err)
	}
	b, err = os.ReadFile(filepath.Join(cache.dir, "GeoIP.dat"))
	if err != nil || string(b) != "old" {
		t.Fatalf("symlink changed cache %s %v", b, err)
	}
}
func TestGeoManualUpdateAndFailure(t *testing.T) {
	runtime, err := os.MkdirTemp("", "rocketscience-core-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(runtime) })
	cache := geoCache{dir: t.TempDir()}
	failUpdate := false
	s := &Service{
		cmd:        &exec.Cmd{},
		runtimeDir: runtime,
		updateGeo: func() error {
			_ = os.WriteFile(filepath.Join(runtime, "geoip.metadb"), []byte("new"), 0600)
			if failUpdate {
				return errors.New("database download blocked")
			}
			return nil
		},
	}
	s.geo.dir = cache.dir
	s.geoSession = geoSession{Version: 1, Runtime: runtime, PID: os.Getpid()}
	if err := s.geo.writeSession(s.geoSession); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateGeo(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(cache.dir, "geoip.metadb"))
	if err != nil || string(b) != "new" {
		t.Fatalf("updated cache: %s %v", b, err)
	}
	failUpdate = true
	if err = s.UpdateGeo(); err == nil || !s.geoDirtyFailed {
		t.Fatal("failed update not recorded")
	}
	if !strings.Contains(err.Error(), "database download blocked") {
		t.Fatalf("download detail lost: %v", err)
	}
	s.cmd = nil
	if err = s.UpdateGeo(); err == nil {
		t.Fatal("update while stopped")
	}
}
func TestGeoFilesSurviveCoreStopAndRestart(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", filepath.Join(t.TempDir(), "missing"))
	s := newTestService(t)
	s.geo.dir = t.TempDir()
	defer s.Shutdown()
	port := freePort(t)
	if err := s.Start(direct, port); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	dir := s.runtimeDir
	s.mu.Unlock()
	if err := os.WriteFile(filepath.Join(dir, "GeoSite.dat"), []byte("test data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("temporary dir not cleaned: %v", err)
	}
	if err := s.Start(direct, port); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	dir = s.runtimeDir
	s.mu.Unlock()
	b, err := os.ReadFile(filepath.Join(dir, "GeoSite.dat"))
	if err != nil || string(b) != "test data" {
		t.Fatalf("restart: %s %v", b, err)
	}
	s.handleUnexpectedExit(errors.New("simulated crash"))
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && s.Status().Running {
		time.Sleep(25 * time.Millisecond)
	}
	if s.Status().Running {
		t.Fatal("crashed core stayed running")
	}
	// wait for cleanup and copy, which follows process state change
	for time.Now().Before(deadline) {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("crashed core did not finish GEO cleanup")
}
