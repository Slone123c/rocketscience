package core

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// connectProxy is a minimal HTTP CONNECT proxy, standing in for a real node.
func connectProxy(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				req, err := http.ReadRequest(bufio.NewReader(conn))
				if err != nil || req.Method != http.MethodConnect {
					return
				}
				upstream, err := net.Dial("tcp", req.Host)
				if err != nil {
					return
				}
				defer upstream.Close()
				fmt.Fprint(conn, "HTTP/1.1 200 Connection established\r\n\r\n")
				go io.Copy(upstream, conn)
				io.Copy(conn, upstream)
			}(conn)
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return ln
}

func TestPreviewDelayDialsNodesWithoutCore(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer target.Close()
	proxy := connectProxy(t)
	closed, _ := net.Listen("tcp", "127.0.0.1:0")
	closedAddr := closed.Addr().(*net.TCPAddr)
	closed.Close()
	port := proxy.Addr().(*net.TCPAddr).Port
	cfg := fmt.Sprintf(`proxies:
  - {name: 东京, type: http, server: 127.0.0.1, port: %d}
  - {name: 失联, type: http, server: 127.0.0.1, port: %d}
proxy-groups:
  - {name: 自动, type: url-test, proxies: [东京]}
  - {name: 选择, type: select, proxies: [自动, 东京, 失联, DIRECT]}
`, port, closedAddr.Port)
	result, err := testPreviewGroup(cfg, "选择", target.URL, 3000)
	if err != nil {
		t.Fatal(err)
	}
	if result["东京"] <= 0 {
		t.Fatalf("reachable node not measured: %v", result)
	}
	if v, ok := result["失联"]; !ok || v != 0 {
		t.Fatalf("unreachable node should time out: %v", result)
	}
	if result["自动"] != -1 || result["DIRECT"] != -1 {
		t.Fatalf("members without a server should report -1: %v", result)
	}
	if _, err := testPreviewGroup(cfg, "缺失", target.URL, 3000); err == nil {
		t.Fatal("missing group accepted")
	}
}
