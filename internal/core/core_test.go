package core

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const direct = "mode: rule\nlog-level: silent\nproxies: []\nproxy-groups: []\nrules:\n  - MATCH,DIRECT\n"

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("go.mod not found")
		}
		wd = parent
	}
}

// copyBundledGeoToDir seeds mihomo GEO files shipped in resources/ so tests that
// enable TUN DNS defaults (geosite:cn policy) do not download GeoSite.dat.
func copyBundledGeoToDir(t *testing.T, dest string) {
	t.Helper()
	root := repoRoot(t)
	for _, name := range []string{"geosite.dat", "geoip.metadb"} {
		src := filepath.Join(root, "resources", name)
		in, err := os.Open(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(dest, 0700); err != nil {
			in.Close()
			t.Fatal(err)
		}
		out, err := os.OpenFile(filepath.Join(dest, name), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			in.Close()
			t.Fatal(err)
		}
		if _, err = io.Copy(out, in); err != nil {
			out.Close()
			in.Close()
			t.Fatal(err)
		}
		if err := out.Close(); err != nil {
			in.Close()
			t.Fatal(err)
		}
		if err := in.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func testGeoDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	copyBundledGeoToDir(t, dir)
	return dir
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	s := New()
	s.proxy = &fakeProxy{}
	s.geo.dir = testGeoDir(t)
	if err := s.SetSystemProxy(true); err != nil {
		t.Fatal(err)
	}
	return s
}
func TestCoreLifecycle(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", filepath.Join(t.TempDir(), "missing"))
	s := newTestService(t)
	defer s.Shutdown()
	p := freePort(t)
	if err := s.Start(direct, p); err != nil {
		t.Fatal(err)
	}
	st := s.Status()
	if !st.Running || st.Version == "" || st.MixedPort != p || st.Error != "" {
		t.Fatalf("unexpected status: %+v", st)
	}
	if _, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(p))); err != nil {
		t.Fatalf("proxy not listening: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer server.Close()
	proxyURL, _ := url.Parse("http://127.0.0.1:" + strconv.Itoa(p))
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	response, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("request through proxy: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", response.StatusCode)
	}
	if err := s.Start(direct, p); err == nil {
		t.Fatal("second start should fail")
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if s.Status().Running {
		t.Fatal("still running")
	}
}
func TestRejectInvalidConfigAndPort(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", filepath.Join(t.TempDir(), "missing"))
	s := newTestService(t)
	defer s.Shutdown()
	if err := s.Start(direct, 0); err == nil {
		t.Fatal("invalid port accepted")
	}
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	if err := s.Start(direct, occupied.Addr().(*net.TCPAddr).Port); err == nil {
		t.Fatal("occupied port accepted")
	}
	if err := s.Start("rules: [", freePort(t)); err == nil {
		t.Fatal("invalid YAML accepted")
	}
	err = s.Start("rules:\n  - NOT-A-RULE,example,DIRECT\n", freePort(t))
	if err == nil || !strings.Contains(err.Error(), "mihomo") {
		t.Fatalf("invalid rule accepted: %v", err)
	}
}

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
