package core

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestInboundOptionsValidationAndPersistence(t *testing.T) {
	p := NewProfiles(t.TempDir())
	if err := p.SetPort(7890); err != nil {
		t.Fatal(err)
	}
	want := InboundOptions{HTTPPort: 17890, SOCKSPort: 17891, Authentication: ptr([]string{"user:pass"})}
	if _, err := p.SetInboundSettings(want); err != nil {
		t.Fatal(err)
	}
	got, err := NewProfiles(p.dir).InboundSettings()
	if err != nil || got.HTTPPort != want.HTTPPort || got.Authentication == nil {
		t.Fatalf("persist: %+v %v", got, err)
	}
	if err = p.SetPort(want.HTTPPort); err == nil {
		t.Fatal("mixed port collided with HTTP")
	}
	for _, bad := range []InboundOptions{{HTTPPort: -1}, {HTTPPort: 65536}, {HTTPPort: 7890}, {HTTPPort: 1234, SOCKSPort: 1234}, {Authentication: ptr([]string{"user:"})}, {Authentication: ptr([]string{"user:one", "user:two"})}} {
		if _, err := p.SetInboundSettings(bad); err == nil {
			t.Fatal("invalid inbound accepted")
		}
	}
	if _, err := p.SetCoreSettings(CoreOptions{LogLevel: ptr("info")}); err != nil {
		t.Fatal(err)
	}
	got, _ = p.InboundSettings()
	if got.HTTPPort != want.HTTPPort {
		t.Fatal("common settings overwrote inbound")
	}
}
func TestInboundOptionsRealCoreListenersAndAuthentication(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO")
	}
	s := newTestService(t)
	s.geo.dir = t.TempDir()
	defer s.Shutdown()
	mixed, httpPort, socksPort := freePort(t), freePort(t), freePort(t)
	for httpPort == mixed {
		httpPort = freePort(t)
	}
	for socksPort == mixed || socksPort == httpPort {
		socksPort = freePort(t)
	}
	options := CoreOptions{Inbound: &InboundOptions{HTTPPort: httpPort, SOCKSPort: socksPort, Authentication: ptr([]string{"user:pass"})}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer server.Close()
	if err := s.start(direct, mixed, nil, LANSettings{Enabled: true, Address: "*"}, options); err != nil {
		t.Fatal(err)
	}
	check := func(address string, want int) {
		t.Helper()
		p, _ := url.Parse(address)
		transport := &http.Transport{Proxy: http.ProxyURL(p), DisableKeepAlives: true}
		defer transport.CloseIdleConnections()
		client := http.Client{Transport: transport, Timeout: 3 * time.Second}
		res, err := client.Get(server.URL)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != want {
			t.Fatalf("%s got %d want %d", p.Host, res.StatusCode, want)
		}
	}
	check(fmt.Sprintf("http://127.0.0.1:%d", mixed), 204)
	check(fmt.Sprintf("http://127.0.0.1:%d", httpPort), 204)
	check(fmt.Sprintf("socks5://127.0.0.1:%d", socksPort), 204)
	interfaces, err := (&Profiles{}).LANInterfaces()
	if err == nil && len(interfaces) > 0 {
		address := interfaces[0].Address
		check(fmt.Sprintf("http://%s:%d", address, httpPort), 407)
		check(fmt.Sprintf("http://user:pass@%s:%d", address, httpPort), 204)
	}
	if ok, err := s.reloadIfRunning(direct); err != nil || !ok {
		t.Fatalf("reload %v %v", ok, err)
	}
	check(fmt.Sprintf("http://127.0.0.1:%d", httpPort), 204)
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	for _, port := range []int{httpPort, socksPort} {
		if conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second); err == nil {
			conn.Close()
			t.Fatal("listener survived stop")
		}
	}
}
func TestInboundOccupiedPortFailsBeforeStart(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	s := newTestService(t)
	defer s.Shutdown()
	err = s.start(direct, freePort(t), nil, LANSettings{}, CoreOptions{Inbound: &InboundOptions{HTTPPort: port}})
	if err == nil || !strings.Contains(err.Error(), fmt.Sprint(port)) || s.Status().Running {
		t.Fatalf("occupied port: %v", err)
	}
}

func TestProfileListenerCollidingWithManagedPorts(t *testing.T) {
	src := "listeners:\n  - name: custom\n    type: mixed\n    port: 7890\nrules: [MATCH,DIRECT]\n"
	if _, err := runtimeConfigWithOptions(src, 7890, 9090, "s", "", LANSettings{}, false, CoreOptions{}); err == nil {
		t.Fatal("accepted profile listener colliding with mixed-port")
	}
	srcInbound := "listeners:\n  - name: custom\n    type: http\n    port: 17890\nrules: [MATCH,DIRECT]\n"
	if _, err := runtimeConfigWithOptions(srcInbound, 7890, 9090, "s", "", LANSettings{}, false, CoreOptions{Inbound: &InboundOptions{HTTPPort: 17890}}); err == nil {
		t.Fatal("accepted profile listener colliding with inbound HTTP port")
	}
	srcCtrl := "listeners:\n  - name: custom\n    type: socks\n    port: 9090\nrules: [MATCH,DIRECT]\n"
	if _, err := runtimeConfigWithOptions(srcCtrl, 7890, 9090, "s", "", LANSettings{}, false, CoreOptions{}); err == nil {
		t.Fatal("accepted profile listener colliding with controller port")
	}
}
