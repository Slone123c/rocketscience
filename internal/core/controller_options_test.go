package core

import (
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestControllerOptionsValidationAndPersistence(t *testing.T) {
	p := NewProfiles(t.TempDir())
	want := ControllerOptions{Address: "127.0.0.1", Port: 19090, Secret: "user key"}
	if _, err := p.SetControllerSettings(want); err != nil {
		t.Fatal(err)
	}
	got, err := NewProfiles(p.dir).ControllerSettings()
	if err != nil || got != want {
		t.Fatalf("persist: %+v %v", got, err)
	}
	if err = p.SetPort(want.Port); err == nil {
		t.Fatal("mixed/controller collision")
	}
	if _, err = p.SetInboundSettings(InboundOptions{HTTPPort: want.Port}); err == nil {
		t.Fatal("HTTP/controller collision")
	}
	for _, bad := range []ControllerOptions{{Address: "*"}, {Address: "192.0.2.1"}, {Address: "https://localhost"}, {Port: -1}, {Port: 65536}, {Secret: "bad\nkey"}} {
		if _, err := p.SetControllerSettings(bad); err == nil {
			t.Fatalf("invalid: %+v", bad)
		}
	}
	if _, err = p.SetCoreSettings(CoreOptions{LogLevel: ptr("info")}); err != nil {
		t.Fatal(err)
	}
	got, _ = p.ControllerSettings()
	if got != want {
		t.Fatal("common settings overwrote controller")
	}
}
func TestControllerOptionsRealCoreReconnection(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", filepath.Join(t.TempDir(), "missing"))
	hosts := []string{"127.0.0.1"}
	interfaces, err := (&Profiles{}).LANInterfaces()
	if err == nil && len(interfaces) > 0 {
		hosts = append(hosts, interfaces[0].Address)
	}
	for _, host := range hosts {
		t.Run(host, func(t *testing.T) {
			s := newTestService(t)
			s.geo.dir = t.TempDir()
			defer s.Shutdown()
			port := freePort(t)
			mixed := freePort(t)
			for mixed == port {
				mixed = freePort(t)
			}
			options := CoreOptions{Controller: &ControllerOptions{Address: host, Port: port, Secret: "controller-test-secret"}}
			if err := s.start(direct, mixed, nil, LANSettings{}, options); err != nil {
				t.Fatal(err)
			}
			expected := net.JoinHostPort(host, fmt.Sprint(port))
			if got := s.Status().Controller; got != expected {
				t.Fatalf("endpoint %q want %q", got, expected)
			}
			client := http.Client{Timeout: 2 * time.Second}
			res, err := client.Get("http://" + expected + "/version")
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			if res.StatusCode != 401 {
				t.Fatalf("unauthenticated status %d", res.StatusCode)
			}
			if _, err := s.Groups(); err != nil {
				t.Fatal(err)
			}
			if err := s.SetMode("direct"); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Connections(); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(4 * time.Second)
			for !s.LiveTelemetry().Connected && time.Now().Before(deadline) {
				time.Sleep(50 * time.Millisecond)
			}
			if !s.LiveTelemetry().Connected {
				t.Fatal("traffic stream did not reconnect to controller")
			}
			if ok, err := s.reloadIfRunning(direct); err != nil || !ok {
				t.Fatalf("reload %v %v", ok, err)
			}
			if st := s.Status(); !st.Running || st.Controller != expected || st.Error != "" {
				t.Fatalf("after reload: %+v", st)
			}
		})
	}
}
func TestControllerOptionsOccupiedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	s := newTestService(t)
	defer s.Shutdown()
	err = s.start(direct, freePort(t), nil, LANSettings{}, CoreOptions{Controller: &ControllerOptions{Address: "127.0.0.1", Port: port, Secret: "secret"}})
	if err == nil || !strings.Contains(err.Error(), "控制接口") || s.Status().Running {
		t.Fatalf("occupied: %v", err)
	}
}

func TestControllerOptionsIPv6BracketsAndZoneID(t *testing.T) {
	p := NewProfiles(t.TempDir())
	for _, raw := range []string{"[::1]", "::1"} {
		got, err := p.SetControllerSettings(ControllerOptions{Address: raw, Port: 19090})
		if err != nil {
			t.Fatalf("SetControllerSettings(%q) failed: %v", raw, err)
		}
		if got.Address != "::1" {
			t.Fatalf("Address(%q) = %q, want \"::1\"", raw, got.Address)
		}
	}
	ifaces, err := net.Interfaces()
	if err != nil || len(ifaces) == 0 {
		t.Fatalf("net.Interfaces: %v (len=%d)", err, len(ifaces))
	}
	zone := ifaces[0].Name
	if _, err := p.SetControllerSettings(ControllerOptions{Address: "fe80::1%" + zone, Port: 19090}); err == nil {
		t.Fatal("accepted link-local IPv6 without secret")
	}
	got, err := p.SetControllerSettings(ControllerOptions{Address: "[fe80::1%" + zone + "]", Port: 19090, Secret: "s"})
	if err != nil {
		t.Fatalf("SetControllerSettings([fe80::1%%%s]) failed: %v", zone, err)
	}
	if got.Address != "fe80::1%"+zone {
		t.Fatalf("got.Address = %q, want %q", got.Address, "fe80::1%"+zone)
	}
	if _, err := p.SetControllerSettings(ControllerOptions{Address: "fe80::1%nonexistent999", Port: 19090, Secret: "s"}); err == nil {
		t.Fatal("accepted non-existent IPv6 Zone ID")
	}
	s := &Service{controllerAddress: net.JoinHostPort(got.Address, "19090")}
	if _, err := http.NewRequest("GET", "http://"+s.controllerAuthority()+"/version", nil); err != nil {
		t.Fatalf("controllerAuthority URL failed: %v (authority=%q)", err, s.controllerAuthority())
	}
}

func TestControllerAuthorityEscapesSpaceInZoneID(t *testing.T) {
	s := &Service{controllerAddress: net.JoinHostPort("fe80::1%Ethernet 3", "19090")}
	req, err := http.NewRequest("GET", "http://"+s.controllerAuthority()+"/version", nil)
	if err != nil {
		t.Fatalf("http.NewRequest failed: %v (authority=%q)", err, s.controllerAuthority())
	}
	if got := req.URL.Hostname(); got != "fe80::1%Ethernet 3" {
		t.Fatalf("URL.Hostname() = %q, want %q", got, "fe80::1%Ethernet 3")
	}
}
