package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestDesktopClipboardReportsNativeFailure(t *testing.T) {
	failure := errors.New("clipboard unavailable")
	d := &Desktop{clipboardWrite: func(string) error { return failure }}
	if err := d.CopyText("example.com"); !errors.Is(err, failure) {
		t.Fatalf("copy failure hidden: %v", err)
	}
	if err := d.CopyTerminalProxy(7890); !errors.Is(err, failure) {
		t.Fatalf("command copy failure hidden: %v", err)
	}
}

func TestDesktopClipboardPreservesAddressesAndValidatesCommandPort(t *testing.T) {
	var writes []string
	d := &Desktop{clipboardWrite: func(text string) error { writes = append(writes, text); return nil }}
	for _, text := range []string{"example.com", "192.168.1.2:7890", "[::1]:7890", "内部域名.example"} {
		if err := d.CopyText(text); err != nil {
			t.Fatal(err)
		}
		if writes[len(writes)-1] != text {
			t.Fatal("address changed during copy")
		}
	}
	count := len(writes)
	if err := d.CopyText(""); err == nil || len(writes) != count {
		t.Fatal("empty address replaced clipboard")
	}
	for _, port := range []int{-1, 0, 65536} {
		if err := d.CopyTerminalProxy(port); err == nil || len(writes) != count {
			t.Fatal("invalid command port copied")
		}
	}
	for _, port := range []int{1, 7890, 65535} {
		if err := d.CopyTerminalProxy(port); err != nil {
			t.Fatal(err)
		}
		command := writes[len(writes)-1]
		if !strings.HasPrefix(command, "export http_proxy=http://127.0.0.1:") || strings.Count(command, fmt.Sprintf(":%d", port)) != 3 || !strings.Contains(command, "all_proxy=socks5://127.0.0.1:") {
			t.Fatalf("incorrect proxy command: %s", command)
		}
	}
}
