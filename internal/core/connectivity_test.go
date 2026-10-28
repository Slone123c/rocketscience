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

func TestConnectivityDoesNotRecommendBuiltInDirectOrReject(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	s := metadataService(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/version":
			_, _ = w.Write([]byte(`{"version":"test"}`))
		case r.Method == "GET" && r.URL.Path == "/proxies":
			_, _ = w.Write([]byte(`{"proxies":{"choose":{"type":"Selector","now":"dead","all":["dead","DIRECT","REJECT","backup"]}}}`))
		case r.Method == "GET" && r.URL.Path == "/group/choose/delay":
			_, _ = w.Write([]byte(`{"DIRECT":1,"backup":45}`))
		}
	})
	s.mixedPort = freePort(t)
	result, err := s.CheckConnectivity(target.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Recommendations) != 1 || result.Recommendations[0].Node != "backup" {
		t.Fatalf("expected backup node recommendation (skipping DIRECT), got: %+v", result.Recommendations)
	}
}
