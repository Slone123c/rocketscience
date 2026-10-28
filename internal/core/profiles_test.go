package core

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProfilesPreserveValidConfiguration(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO to test a real core")
	}
	dir := t.TempDir()
	p := NewProfiles(dir)
	first, err := p.Save(Profile{Name: "Test", Source: "file", Content: direct}, "")
	if err != nil {
		t.Fatal(err)
	}
	id := first.ActiveID
	if id == "" {
		t.Fatal("first profile was not activated")
	}
	v := first.Profiles[0]
	v.Content = "rules: ["
	if _, err = p.Save(v, v.UpdatedAt.Format(time.RFC3339Nano)); err == nil {
		t.Fatal("invalid update accepted")
	}
	again, err := NewProfiles(dir).State()
	if err != nil || again.Profiles[0].Content != direct || again.ActiveID != id {
		t.Fatalf("saved state damaged: %+v %v", again, err)
	}
	if _, err = p.Delete(id); err == nil {
		t.Fatal("active configuration was deleted")
	}
	if err = p.SetPort(9012); err != nil {
		t.Fatal(err)
	}
	again, err = NewProfiles(dir).State()
	if err != nil || again.MixedPort != 9012 {
		t.Fatal("port did not persist")
	}
	info, err := os.Stat(filepath.Join(dir, "profiles.json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("unsafe permissions: %v %v", info, err)
	}
}
func TestFetchSubscription(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO to test a real core")
	}
	p := NewProfiles(t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "clash.meta" {
			t.Error("missing user agent")
		}
		if r.URL.Path == "/bad" {
			w.Write([]byte("rules: ["))
			return
		}
		w.Write([]byte(direct))
	}))
	defer server.Close()
	v, err := p.Fetch(server.URL)
	if err != nil || v.Content != direct {
		t.Fatalf("valid subscription: %v", err)
	}
	if _, err = p.Fetch(server.URL + "/bad"); err == nil {
		t.Fatal("invalid subscription accepted")
	}
	if _, err = p.Fetch("file:///tmp/foo"); err == nil || !strings.Contains(err.Error(), "HTTP") {
		t.Fatal("invalid URL accepted")
	}
	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(direct))
	}))
	defer tlsServer.Close()
	if _, err = p.Fetch(tlsServer.URL); err == nil || (!strings.Contains(err.Error(), "certificate") && !strings.Contains(err.Error(), "x509")) {
		t.Fatalf("expected TLS certificate detail in fetch error, got: %v", err)
	}
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "ftp://example.com/sub.yaml", http.StatusFound)
	}))
	defer redirectServer.Close()
	if _, err = p.Fetch(redirectServer.URL); err == nil || !strings.Contains(err.Error(), "不支持的重定向协议") {
		t.Fatalf("expected redirect scheme error, got: %v", err)
	}
}
