package core

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestConnectivity(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO to test a real core")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer server.Close()
	s := newTestService(t)
	defer s.Shutdown()
	result, err := s.CheckConnectivity(server.URL)
	if err != nil || result.Core {
		t.Fatal("stopped core should fail first step")
	}
	if err = s.Start(direct, freePort(t)); err != nil {
		t.Fatal(err)
	}
	result, err = s.CheckConnectivity(server.URL)
	if err != nil || !result.Core || !result.Request {
		t.Fatalf("check failed: %+v %v", result, err)
	}
	if _, err = s.CheckConnectivity("file:///tmp/x"); err == nil {
		t.Fatal("bad URL accepted")
	}
}
