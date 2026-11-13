package core

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
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
	withAutomaticSystemProxy(t)
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
