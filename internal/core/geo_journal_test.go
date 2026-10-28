package core

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func geoRuntimeFixture(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "rocketscience-core-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err = os.WriteFile(filepath.Join(dir, "GeoSite.dat"), []byte("safe"), 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}
func TestGeoJournalRecoveryAndAmbiguity(t *testing.T) {
	cache := geoCache{dir: t.TempDir()}
	safe, dirty, automatic, live := geoRuntimeFixture(t), geoRuntimeFixture(t), geoRuntimeFixture(t), geoRuntimeFixture(t)
	for _, session := range []geoSession{
		{Version: 1, Runtime: safe, PID: 1 << 30},
		{Version: 1, Runtime: dirty, PID: 1 << 30, Dirty: true},
		{Version: 1, Runtime: automatic, PID: 1 << 30, AutoUpdate: true},
		{Version: 1, Runtime: live, PID: os.Getpid()},
	} {
		if err := cache.writeSession(session); err != nil {
			t.Fatal(err)
		}
	}
	warnings, err := cache.recover("")
	if err != nil || len(warnings) != 3 {
		t.Fatalf("recovery: %v %v", warnings, err)
	}
	data, err := os.ReadFile(filepath.Join(cache.dir, "GeoSite.dat"))
	if err != nil || string(data) != "safe" {
		t.Fatalf("recovered: %s %v", data, err)
	}
	if _, err := os.Stat(safe); !os.IsNotExist(err) {
		t.Fatalf("safe runtime not removed: %v", err)
	}
	for _, dir := range []string{dirty, automatic, live} {
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("ambiguous session removed: %v", err)
		}
	}
	warnings, err = cache.recover(live)
	if err != nil || len(warnings) != 2 {
		t.Fatalf("exclude active: %v %v", warnings, err)
	}
}
func TestGeoJournalCannotEscapeTemp(t *testing.T) {
	cache := geoCache{dir: t.TempDir()}
	bad := filepath.Join(t.TempDir(), "rocketscience-core-malicious")
	if err := os.Mkdir(bad, 0700); err != nil {
		t.Fatal(err)
	}
	if err := cache.writeSession(geoSession{Version: 1, Runtime: bad, PID: 1 << 30}); err != nil {
		t.Fatal(err)
	}
	warnings, err := cache.recover("")
	if err != nil || len(warnings) != 1 || !strings.Contains(warnings[0], "需要检查") {
		t.Fatalf("invalid runtime: %v %v", warnings, err)
	}
	if _, err := os.Stat(bad); err != nil {
		t.Fatal("invalid runtime was removed")
	}
}
func TestGeoCrashFixture(t *testing.T) {
	cacheDir := os.Getenv("ROCKET_GEO_CRASH_CACHE")
	if cacheDir == "" {
		return
	}
	s := New()
	s.proxy = &fakeProxy{}
	s.geo.dir = cacheDir
	if err := s.SetSystemProxy(true); err != nil {
		os.Exit(10)
	}
	if err := s.Start(direct, freePort(t)); err != nil {
		os.Exit(11)
	}
	if err := os.WriteFile(filepath.Join(s.runtimeDir, "GeoIP.dat"), []byte("after crash"), 0600); err != nil {
		os.Exit(12)
	}
	os.Exit(0)
}
func TestGeoRecoveryAfterAppExit(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO")
	}
	cacheDir := t.TempDir()
	child := exec.Command(os.Args[0], "-test.run=^TestGeoCrashFixture$")
	child.Env = append(os.Environ(), "ROCKET_GEO_CRASH_CACHE="+cacheDir)
	if out, err := child.CombinedOutput(); err != nil {
		t.Fatalf("fixture: %s %v", out, err)
	}
	cache := geoCache{dir: cacheDir}
	entries, err := os.ReadDir(filepath.Join(cacheDir, "sessions"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("journal: %v %v", entries, err)
	}
	data, err := os.ReadFile(filepath.Join(cacheDir, "sessions", entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	var session geoSession
	if err = json.Unmarshal(data, &session); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for processAlive(session.PID) && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	if processAlive(session.PID) {
		t.Fatal("watchdog did not stop the fixture core after the app exited")
	}
	warnings, err := cache.recover("")
	if err != nil || len(warnings) > 0 {
		t.Fatalf("recovery: %v %v", warnings, err)
	}
	data, err = os.ReadFile(filepath.Join(cacheDir, "GeoIP.dat"))
	if err != nil || string(data) != "after crash" {
		t.Fatalf("recovered data %s %v", data, err)
	}
}

func TestGeoJournalMarksAutomaticUpdatesBeforeReload(t *testing.T) {
	runtime := geoRuntimeFixture(t)
	s := New()
	s.geo.dir = t.TempDir()
	s.geoSession = geoSession{Version: 1, Runtime: runtime, PID: os.Getpid()}
	if err := s.geo.writeSession(s.geoSession); err != nil {
		t.Fatal(err)
	}
	if err := s.markGeoAuto([]byte("geo-auto-update: true\n")); err != nil {
		t.Fatal(err)
	}
	if !s.geoSession.AutoUpdate {
		t.Fatal("automatic update was not journaled")
	}
	if err := s.markGeoAuto([]byte("geo-auto-update: false\n")); err != nil {
		t.Fatal(err)
	}
	if !s.geoSession.AutoUpdate {
		t.Fatal("potential interrupted update was forgotten")
	}
	path, err := s.geo.journalPath(runtime)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var session geoSession
	if err = json.Unmarshal(data, &session); err != nil || !session.AutoUpdate {
		t.Fatalf("journal: %+v %v", session, err)
	}
}
