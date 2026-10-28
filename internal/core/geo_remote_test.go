package core

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGeoRuntimeValidationCancellation(t *testing.T) {
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := validateRuntimeWithContext(ctx, bin, t.TempDir(), "unused"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation ignored: %v", err)
	}
	ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if err := validateRuntimeWithContext(ctx, bin, t.TempDir(), "unused"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline ignored: %v", err)
	}
}

// Opt-in external integration: uses official mihomo defaults, temporary data
// directories and fake system takeover. It never changes host proxy settings.
func TestGeoRemoteDownloadAndOfflineReuse(t *testing.T) { verifyGeoRemote(t, false) }

func TestGeoRemoteDATDownloadAndOfflineReuse(t *testing.T) { verifyGeoRemote(t, true) }

func verifyGeoRemote(t *testing.T, dataMode bool) {
	t.Helper()
	if os.Getenv("ROCKETSCIENCE_VERIFY_REMOTE_GEO") != "1" || os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("opt in with ROCKETSCIENCE_VERIFY_REMOTE_GEO=1 and ROCKETSCIENCE_MIHOMO")
	}
	s := newTestService(t)
	s.geo.dir = t.TempDir()
	defer s.Shutdown()
	config := "mode: rule\nlog-level: info\ngeodata-mode: false\ngeo-auto-update: false\nrules:\n  - GEOIP,CN,DIRECT\n  - MATCH,DIRECT\n"
	if dataMode {
		config = "mode: rule\nlog-level: info\ngeodata-mode: true\ngeo-auto-update: false\nrules:\n  - GEOIP,CN,DIRECT\n  - GEOSITE,CN,DIRECT\n  - MATCH,DIRECT\n"
	}
	port := freePort(t)
	if err := s.Start(config, port); err != nil {
		t.Fatalf("initial remote GEO load: %v", err)
	}
	if err := s.UpdateGeo(); err != nil {
		t.Fatalf("remote update: %v", err)
	}
	entries, err := os.ReadDir(s.geo.dir)
	if err != nil {
		t.Fatal(err)
	}
	hashes := map[string][32]byte{}
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !isGeoName(entry.Name()) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.geo.dir, entry.Name()))
		if err != nil || len(data) == 0 {
			t.Fatalf("empty/invalid cached database: %v", err)
		}
		hashes[entry.Name()] = sha256.Sum256(data)
		t.Logf("cached %s: %d bytes", entry.Name(), len(data))
	}
	if len(hashes) == 0 {
		t.Fatal("remote update did not persist a GEO database")
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	unavailable := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer unavailable.Close()
	offlineConfig := config + fmt.Sprintf("geox-url:\n  mmdb: %q\n  geoip: %q\n  geosite: %q\n", unavailable.URL+"/database", unavailable.URL+"/database", unavailable.URL+"/database")
	if err := s.Start(offlineConfig, port); err != nil {
		t.Fatalf("cached offline startup: %v", err)
	}
	if requests.Load() != 0 {
		t.Fatal("restart downloaded despite a valid cached database")
	}
	s.mu.Lock()
	runtimeDir := s.runtimeDir
	s.mu.Unlock()
	for name, hash := range hashes {
		data, err := os.ReadFile(filepath.Join(runtimeDir, name))
		if err != nil || sha256.Sum256(data) != hash {
			t.Fatalf("database not reused: %s %v", name, err)
		}
	}
	if err := s.UpdateGeo(); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("upstream failure detail missing: %v", err)
	}
	if requests.Load() == 0 {
		t.Fatal("failure branch never attempted download")
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	for name, hash := range hashes {
		data, err := os.ReadFile(filepath.Join(s.geo.dir, name))
		if err != nil || sha256.Sum256(data) != hash {
			t.Fatalf("failed download damaged last good cache: %s %v", name, err)
		}
	}
}
