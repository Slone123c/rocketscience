//go:build darwin

package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// TestTunFakeProcess is a subprocess used to exercise lifecycle logic without
// requesting administrator authorization or changing the host's routes.
func TestTunFakeProcess(t *testing.T) {
	dir := os.Getenv("ROCKET_TUN_FAKE_DIR")
	if dir == "" {
		return
	}
	b, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		os.Exit(2)
	}
	var cfg struct {
		Controller string `yaml:"external-controller"`
	}
	if yaml.Unmarshal(b, &cfg) != nil {
		os.Exit(3)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"version": "fake"})
	})
	mux.HandleFunc("/configs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"mode": "rule", "tun": map[string]any{"enable": true}})
	})
	mux.HandleFunc("/proxies", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"proxies": map[string]any{}})
	})
	server := &http.Server{Addr: cfg.Controller, Handler: mux}
	go func() { _ = server.ListenAndServe() }()
	for {
		b, err := os.ReadFile(filepath.Join(dir, "heartbeat"))
		if err != nil || strings.TrimSpace(string(b)) == "stop" {
			_ = server.Close()
			os.Exit(0)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
func fakeTunLaunch(_ string, dir string) (*tunLauncher, error) {
	if err := os.WriteFile(filepath.Join(dir, "heartbeat"), []byte("running"), 0600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "tun-core.log"), nil, 0600); err != nil {
		return nil, err
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestTunFakeProcess$")
	cmd.Env = append(os.Environ(), "ROCKET_TUN_FAKE_DIR="+dir)
	l := &tunLauncher{cmd: cmd, dir: dir, done: make(chan struct{}), exited: make(chan struct{})}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go l.heartbeat()
	return l, nil
}
func TestTunTakeoverLifecycle(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO")
	}
	s := New()
	proxy := &fakeProxy{}
	s.proxy = proxy
	s.tunLaunch = fakeTunLaunch
	s.tunReady = func(map[string]bool) bool { return true }
	defer s.Shutdown()
	if err := s.SetTun(true); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(direct, freePort(t)); err != nil {
		t.Fatal(err)
	}
	if st := s.Status(); !st.Running || !st.Tun || st.SystemProxy {
		t.Fatalf("TUN only: %+v", st)
	}
	if err := s.SetSystemProxy(true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSystemProxy(false); err != nil {
		t.Fatal(err)
	}
	if st := s.Status(); !st.Running || !st.Tun || st.SystemProxy {
		t.Fatalf("system proxy off stopped TUN: %+v", st)
	}
	if err := s.SetTun(false); err != nil {
		t.Fatal(err)
	}
	if st := s.Status(); st.Running || st.Tun {
		t.Fatalf("last takeover still active: %+v", st)
	}
}
func TestTunRestartAndAuthorizationRollback(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO")
	}
	s := newTestService(t)
	proxy := s.proxy.(*fakeProxy)
	s.tunReady = func(map[string]bool) bool { return true }
	defer s.Shutdown()
	if err := s.Start(direct, freePort(t)); err != nil {
		t.Fatal(err)
	}
	s.tunLaunch = func(string, string) (*tunLauncher, error) { return nil, errors.New("authorization cancelled") }
	if err := s.SetTun(true); err == nil {
		t.Fatal("cancelled authorization accepted")
	}
	if st := s.Status(); !st.Running || !st.SystemProxy || st.Tun || !proxy.pending {
		t.Fatalf("failed rollback: %+v", st)
	}
	s.tunLaunch = fakeTunLaunch
	if err := s.SetTun(true); err != nil {
		t.Fatal(err)
	}
	if st := s.Status(); !st.Running || !st.SystemProxy || !st.Tun {
		t.Fatalf("enabled: %+v", st)
	}
	if err := s.SetTun(false); err != nil {
		t.Fatal(err)
	}
	if st := s.Status(); !st.Running || !st.SystemProxy || st.Tun {
		t.Fatalf("disabled: %+v", st)
	}
}
func TestTunSettingsAndConfig(t *testing.T) {
	s := New()
	p := NewProfilesWithService(t.TempDir(), s)
	if err := p.ChangeTun(true); err != nil {
		t.Fatal(err)
	}
	if v, err := NewProfiles(p.dir).TunEnabled(); err != nil || !v {
		t.Fatalf("preference: %v %v", v, err)
	}
	b, err := runtimeConfigWithTakeovers(direct, 7890, 9999, "secret", "", LANSettings{}, true)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = yaml.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	tun := doc["tun"].(map[string]any)
	if tun["enable"] != true || tun["auto-route"] != true {
		t.Fatal(fmt.Sprintf("tun config: %v", tun))
	}
	if q, err := shellArgument("/tmp/a'b"); err != nil || q != "'/tmp/a'\\''b'" {
		t.Fatalf("shell escaping: %q %v", q, err)
	}
}
