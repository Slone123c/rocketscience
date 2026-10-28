package core

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestLANSettingsAndRuntimeConfig(t *testing.T) {
	p := NewProfiles(t.TempDir())
	current, err := p.LANSettings()
	if err != nil || current.Enabled || current.Address != "*" {
		t.Fatalf("defaults: %+v %v", current, err)
	}
	if _, err = p.SetLANSettings(LANSettings{Enabled: true, Address: "127.0.0.1"}); err == nil {
		t.Fatal("loopback accepted as LAN")
	}
	if _, err = p.SetLANSettings(LANSettings{Enabled: true, Address: "203.0.113.42"}); err == nil {
		t.Fatal("unavailable address accepted")
	}
	if _, err = p.SetLANSettings(LANSettings{Enabled: true, Address: "*"}); err != nil {
		t.Fatal(err)
	}
	current, err = NewProfiles(p.dir).LANSettings()
	if err != nil || !current.Enabled || current.Address != "*" {
		t.Fatalf("saved: %+v %v", current, err)
	}
	for _, tc := range []struct {
		name         string
		config       LANSettings
		wantLan      bool
		wantBind     string
		wantListener bool
	}{
		{"off", LANSettings{}, false, "127.0.0.1", false},
		{"all", LANSettings{Enabled: true, Address: "*"}, true, "*", false},
		{"specific", LANSettings{Enabled: true, Address: "192.0.2.1"}, false, "127.0.0.1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, err := runtimeConfigWithLAN("authentication: [user:pass]\nrules: [MATCH,DIRECT]\n", 7890, 9999, "secret", "rule", tc.config)
			if err != nil {
				t.Fatal(err)
			}
			var config map[string]any
			if err = yaml.Unmarshal(b, &config); err != nil {
				t.Fatal(err)
			}
			if config["allow-lan"] != tc.wantLan || config["bind-address"] != tc.wantBind {
				t.Fatalf("listener: %+v", config)
			}
			listeners, ok := config["listeners"].([]any)
			if (ok && len(listeners) > 0) != tc.wantListener {
				t.Fatalf("listeners: %+v", config["listeners"])
			}
			if tc.wantListener {
				listener := listeners[0].(map[string]any)
				if listener["listen"] != "192.0.2.1" || listener["port"] != 7890 {
					t.Fatalf("listener: %+v", listener)
				}
			}
			if tc.config.Enabled && !strings.Contains(string(b), "127.0.0.0/8") {
				t.Fatal("authenticated profile broke system proxy")
			}
		})
	}
	_, err = runtimeConfigWithLAN("listeners:\n  - name: rocketscience-lan\n    type: mixed\n    port: 9000\n", 7890, 9999, "secret", "", LANSettings{Enabled: true, Address: "192.0.2.1"})
	if err == nil {
		t.Fatal("accepted conflicting listener name")
	}
}

func TestLANSharingRealCore(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO")
	}
	p := NewProfiles(t.TempDir())
	interfaces, err := p.LANInterfaces()
	if err != nil || len(interfaces) == 0 {
		t.Skip("no active IPv4 interface")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer server.Close()
	for _, address := range []string{"*", interfaces[0].Address} {
		t.Run(address, func(t *testing.T) {
			s := newTestService(t)
			defer s.Shutdown()
			port := freePort(t)
			if err := s.start(direct, port, nil, LANSettings{Enabled: true, Address: address}, CoreOptions{}); err != nil {
				t.Fatal(err)
			}
			if running, err := s.reloadIfRunning(direct); err != nil || !running {
				t.Fatalf("reload: %v %v", running, err)
			}
			for _, host := range []string{"127.0.0.1", interfaces[0].Address} {
				proxyURL, _ := url.Parse("http://" + net.JoinHostPort(host, strconv.Itoa(port)))
				transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
				client := http.Client{Transport: transport, Timeout: 3 * time.Second}
				var response *http.Response
				var err error
				for attempt := 0; attempt < 20; attempt++ {
					response, err = client.Get(server.URL)
					if err == nil {
						break
					}
					time.Sleep(50 * time.Millisecond)
				}
				if err != nil {
					t.Fatal(fmt.Errorf("proxy %s: %w", host, err))
				}
				response.Body.Close()
				transport.CloseIdleConnections()
				if response.StatusCode != 204 {
					t.Fatalf("proxy %s: %s", host, response.Status)
				}
			}
			if _, err := NewProfilesWithService(t.TempDir(), s).SetLANSettings(LANSettings{}); err == nil {
				t.Fatal("changed setting while running")
			}
			if err := s.Stop(); err != nil {
				t.Fatal(err)
			}
			conn, err := net.DialTimeout("tcp", net.JoinHostPort(interfaces[0].Address, strconv.Itoa(port)), 300*time.Millisecond)
			if err == nil {
				conn.Close()
				t.Fatal("LAN listener survived core stop")
			}
		})
	}
}
