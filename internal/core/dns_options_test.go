package core

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
	"gopkg.in/yaml.v3"
)

func TestDNSOptionsMergeAndValidation(t *testing.T) {
	dns := DNSOptions{Enabled: ptr(true), EnhancedMode: ptr("fake-ip"), FakeIPRange: ptr("198.18.0.1/16"), DefaultNameservers: ptr([]string{"1.1.1.1"}), Nameservers: ptr([]string{"https://dns.example/dns-query"}), Fallback: ptr([]string{})}
	opts := CoreOptions{DNS: &dns}
	source := "dns:\n  enable: false\n  nameserver-policy:\n    example.com: 1.1.1.1\n  fallback: [8.8.8.8]\nrules: [MATCH,DIRECT]\n"
	b, err := runtimeConfigWithOptions(source, 7890, 9999, "secret", "", LANSettings{}, false, opts)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err = yaml.Unmarshal(b, &cfg); err != nil {
		t.Fatal(err)
	}
	got := cfg["dns"].(map[string]any)
	if got["enable"] != true || got["enhanced-mode"] != "fake-ip" || got["nameserver-policy"] == nil || len(got["fallback"].([]any)) != 0 {
		t.Fatalf("dns merge: %+v", got)
	}
	for _, bad := range []DNSOptions{{EnhancedMode: ptr("bad")}, {FakeIPRange: ptr("2001:db8::/32")}, {DefaultNameservers: ptr([]string{"dns.example"})}, {Nameservers: ptr([]string{})}, {Nameservers: ptr([]string{" bad "})}} {
		if err := validateDNSOptions(bad); err == nil {
			t.Fatalf("accepted invalid: %+v", bad)
		}
	}
	p := NewProfiles(t.TempDir())
	if _, err = p.SetCoreSettings(CoreOptions{LogLevel: ptr("warning")}); err != nil {
		t.Fatal(err)
	}
	if _, err = p.SetDNSSettings(dns); err != nil {
		t.Fatal(err)
	}
	// The common settings editor cannot overwrite DNS even if it holds stale data.
	if _, err = p.SetCoreSettings(CoreOptions{LogLevel: ptr("error")}); err != nil {
		t.Fatal(err)
	}
	loaded, err := NewProfiles(p.dir).DNSSettings()
	if err != nil || loaded.FakeIPRange == nil || *loaded.FakeIPRange != "198.18.0.1/16" {
		t.Fatalf("persisted: %+v %v", loaded, err)
	}
	options, err := p.CoreSettings()
	if err != nil || options.LogLevel == nil || *options.LogLevel != "error" {
		t.Fatalf("other settings: %+v %v", options, err)
	}
}
func dnsFixture(t *testing.T) (string, func()) {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		buf := make([]byte, 4096)
		for {
			n, peer, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			var query dnsmessage.Message
			if query.Unpack(buf[:n]) != nil || len(query.Questions) == 0 {
				continue
			}
			q := query.Questions[0]
			answer := dnsmessage.Message{Header: dnsmessage.Header{ID: query.Header.ID, Response: true, RecursionAvailable: true}, Questions: query.Questions}
			if q.Type == dnsmessage.TypeA {
				answer.Answers = []dnsmessage.Resource{{Header: dnsmessage.ResourceHeader{Name: q.Name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET, TTL: 60}, Body: &dnsmessage.AResource{A: [4]byte{127, 0, 0, 1}}}}
			}
			data, err := answer.Pack()
			if err == nil {
				_, _ = conn.WriteTo(data, peer)
			}
		}
	}()
	return conn.LocalAddr().String(), func() { _ = conn.Close() }
}
func TestDNSOptionsRealCoreResolutionAndReload(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO")
	}
	nameserver, closeDNS := dnsFixture(t)
	defer closeDNS()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer server.Close()
	_, portString, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	s := newTestService(t)
	defer s.Shutdown()
	p := NewProfilesWithService(t.TempDir(), s)
	port := freePort(t)
	if err = p.StartDraft(direct, port); err != nil {
		t.Fatal(err)
	}
	value := DNSOptions{Enabled: ptr(true), IPv6: ptr(false), EnhancedMode: ptr("redir-host"), DefaultNameservers: ptr([]string{nameserver}), Nameservers: ptr([]string{nameserver})}
	if _, err = p.SetDNSSettings(value); err != nil {
		t.Fatal(err)
	}
	check := func() {
		t.Helper()
		proxy, _ := url.Parse("http://127.0.0.1:" + strconv.Itoa(port))
		transport := &http.Transport{Proxy: http.ProxyURL(proxy), DisableKeepAlives: true}
		defer transport.CloseIdleConnections()
		client := http.Client{Transport: transport, Timeout: 4 * time.Second}
		res, err := client.Get("http://rocket-test.invalid:" + portString + "/test")
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 204 {
			t.Fatalf("DNS request: %s", res.Status)
		}
	}
	check()
	if ok, err := s.reloadIfRunning(direct); err != nil || !ok {
		t.Fatalf("reload %v %v", ok, err)
	}
	check()
	old, err := p.DNSSettings()
	if err != nil {
		t.Fatal(err)
	}
	bad := value
	bad.FakeIPRange = ptr("bad")
	if _, err = p.SetDNSSettings(bad); err == nil {
		t.Fatal("accepted invalid DNS")
	}
	after, err := p.DNSSettings()
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(old)
	b, _ := json.Marshal(after)
	if string(a) != string(b) {
		t.Fatal("failed validation changed saved DNS")
	}
	check()
}
