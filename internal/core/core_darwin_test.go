//go:build darwin

package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEarlyCoreExitSurfacesLogTailAndCleanLookPath(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", filepath.Join(t.TempDir(), "missing"))
	s := newTestService(t)
	defer s.Shutdown()
	s.tunLaunch = func(_ string, dir string) (*tunLauncher, error) {
		if err := os.WriteFile(filepath.Join(dir, "tun-core.log"), []byte("level=fatal msg=\"bind address already in use: 127.0.0.1:7890\"\n"), 0600); err != nil {
			return nil, err
		}
		cmd := exec.Command("/usr/bin/false")
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		exited := make(chan struct{})
		close(exited)
		return &tunLauncher{cmd: cmd, dir: dir, done: make(chan struct{}), exited: exited}, nil
	}
	if err := s.SetTun(true); err != nil {
		t.Fatal(err)
	}
	err := s.Start(direct, freePort(t))
	if err == nil || !strings.Contains(err.Error(), "bind address already in use") {
		t.Fatalf("expected early exit error to surface log tail, got: %v", err)
	}
	if s.Status().Error != "" && strings.Contains(s.Status().Error, "exit status 1") && !strings.Contains(s.Status().Error, "bind address already in use") {
		t.Fatalf("status error did not include log tail: %+v", s.Status())
	}
}

func TestStopCancelsInFlightStartImmediately(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", filepath.Join(t.TempDir(), "missing"))
	s := newTestService(t)
	defer s.Shutdown()
	launched := make(chan struct{})
	s.tunReady = func(map[string]bool) bool { return false }
	s.tunLaunch = func(core, dir string) (*tunLauncher, error) {
		l, err := fakeTunLaunch(core, dir)
		if err == nil {
			close(launched)
		}
		return l, err
	}
	if err := s.SetTun(true); err != nil {
		t.Fatal(err)
	}
	startDone := make(chan error, 1)
	go func() {
		startDone <- s.Start(direct, freePort(t))
	}()
	select {
	case <-launched:
	case <-time.After(5 * time.Second):
		t.Fatal("tunLaunch did not start")
	}
	time.Sleep(100 * time.Millisecond)
	stopStart := time.Now()
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if elapsed := time.Since(stopStart); elapsed > 2*time.Second {
		t.Fatalf("Stop blocked on in-flight Start for %v (expected < 2s)", elapsed)
	}
	if err := <-startDone; err == nil || !strings.Contains(err.Error(), "已取消") {
		t.Fatalf("expected cancelled start error, got: %v", err)
	}
}

func TestConcurrentSystemProxyReloadAndWiFiCheck(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", t.TempDir()+"/missing")
	p := &fakeProxy{}
	s := New()
	s.geo.dir = testGeoDir(t)
	s.proxy = p
	defer s.Shutdown()
	if err := s.SetTun(true); err != nil {
		t.Fatal(err)
	}
	s.tunLaunch = fakeTunLaunch
	s.tunReady = func(map[string]bool) bool { return true }
	if err := s.Start(direct, freePort(t)); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 6)
	for i := 0; i < 3; i++ {
		enable := i%2 == 0
		go func(en bool) {
			done <- s.SetSystemProxy(en)
		}(enable)
		go func() {
			_, err := s.reloadIfRunning(direct)
			done <- err
		}()
	}
	for i := 0; i < 6; i++ {
		if err := <-done; err != nil {
			t.Fatalf("concurrent SetSystemProxy / reloadIfRunning error: %v", err)
		}
	}
	if err := s.SetSystemProxy(false); err != nil {
		t.Fatal(err)
	}
	st := s.Status()
	if st.SystemProxy || p.pending {
		t.Fatalf("expected system proxy to be off and restored after disabling, got status=%+v pending=%v", st, p.pending)
	}
}
