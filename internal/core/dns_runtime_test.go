package core

import (
	"fmt"
	"net"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

func freeDNSAddress(t *testing.T) string {
	t.Helper()
	udp, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	address := udp.LocalAddr().String()
	tcp, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	tcp.Close()
	return address
}

func queryRuntimeDNS(t *testing.T, address, host string, kind dnsmessage.Type) dnsmessage.Message {
	t.Helper()
	name, err := dnsmessage.NewName(host)
	if err != nil {
		t.Fatal(err)
	}
	message := dnsmessage.Message{Header: dnsmessage.Header{ID: 42, RecursionDesired: true}, Questions: []dnsmessage.Question{{Name: name, Type: kind, Class: dnsmessage.ClassINET}}}
	packet, err := message.Pack()
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.Dial("udp", address)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write(packet); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("DNS query %s: %v", host, err)
	}
	var answer dnsmessage.Message
	if err := answer.Unpack(buf[:n]); err != nil {
		t.Fatal(err)
	}
	if answer.Header.ID != 42 || !answer.Header.Response || answer.Header.RCode != dnsmessage.RCodeSuccess {
		t.Fatalf("invalid response: %+v", answer.Header)
	}
	return answer
}

func assertDNSPortsReleased(t *testing.T, address string) {
	t.Helper()
	udp, err := net.ListenPacket("udp", address)
	if err != nil {
		t.Fatalf("UDP DNS port not released: %v", err)
	}
	defer udp.Close()
	tcp, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("TCP DNS port not released: %v", err)
	}
	tcp.Close()
}

func TestDNSRuntimeModesIPv6AndListenerLifecycle(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", t.TempDir()+"/missing")
	upstream, closeUpstream := dnsFixture(t)
	defer closeUpstream()
	address := freeDNSAddress(t)
	s := newTestService(t)
	defer s.Shutdown()
	p := NewProfilesWithService(t.TempDir(), s)
	content := direct + fmt.Sprintf("dns:\n  enable: false\n  listen: %q\n", address)
	if err := p.StartDraft(content, freePort(t)); err != nil {
		t.Fatal(err)
	}
	options := DNSOptions{Enabled: ptr(true), IPv6: ptr(false), EnhancedMode: ptr("redir-host"), DefaultNameservers: ptr([]string{upstream}), Nameservers: ptr([]string{upstream})}
	if _, err := p.SetDNSSettings(options); err != nil {
		t.Fatal(err)
	}
	response := queryRuntimeDNS(t, address, "dns-a.rocketscience.example.", dnsmessage.TypeA)
	if len(response.Answers) != 1 {
		t.Fatalf("upstream A not returned: %+v", response.Answers)
	}
	if answer, ok := response.Answers[0].Body.(*dnsmessage.AResource); !ok || answer.A != [4]byte{127, 0, 0, 1} {
		t.Fatal("DNS ignored configured upstream")
	}
	response = queryRuntimeDNS(t, address, "dns-v6-disabled.rocketscience.example.", dnsmessage.TypeAAAA)
	if len(response.Answers) != 0 {
		t.Fatal("disabled IPv6 returned AAAA")
	}
	options.IPv6 = ptr(true)
	if _, err := p.SetDNSSettings(options); err != nil {
		t.Fatal(err)
	}
	response = queryRuntimeDNS(t, address, "dns-v6-enabled.rocketscience.example.", dnsmessage.TypeAAAA)
	if len(response.Answers) != 1 {
		t.Fatalf("enabled IPv6 did not resolve AAAA: %+v", response.Answers)
	}
	if answer, ok := response.Answers[0].Body.(*dnsmessage.AAAAResource); !ok || answer.AAAA != [16]byte{15: 1} {
		t.Fatal("AAAA result differs from configured upstream")
	}
	options.EnhancedMode = ptr("fake-ip")
	options.FakeIPRange = ptr("198.18.0.1/16")
	if _, err := p.SetDNSSettings(options); err != nil {
		t.Fatal(err)
	}
	response = queryRuntimeDNS(t, address, "dns-fake.rocketscience.example.com.", dnsmessage.TypeA)
	if len(response.Answers) != 1 {
		t.Fatalf("fake IP not returned: %+v", response.Answers)
	}
	answer, ok := response.Answers[0].Body.(*dnsmessage.AResource)
	_, fakeRange, _ := net.ParseCIDR("198.18.0.0/16")
	if !ok || !fakeRange.Contains(net.IP(answer.A[:])) {
		t.Fatalf("address outside configured fake IP range: %+v", response.Answers)
	}
	options.Enabled = ptr(false)
	if _, err := p.SetDNSSettings(options); err != nil {
		t.Fatal(err)
	}
	assertDNSPortsReleased(t, address)
	options.Enabled = ptr(true)
	options.EnhancedMode = ptr("redir-host")
	if _, err := p.SetDNSSettings(options); err != nil {
		t.Fatal(err)
	}
	queryRuntimeDNS(t, address, "dns-reenabled.rocketscience.example.", dnsmessage.TypeA)
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	assertDNSPortsReleased(t, address)
}
