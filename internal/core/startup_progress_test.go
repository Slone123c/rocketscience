//go:build !windows

package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartupValidationReportsProgressAndCanBeCancelled(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "validating")
	bin := filepath.Join(dir, "slow-core")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nif [ \"$1\" = \"-t\" ]; then printf ready > \"$RS_TEST_START_MARKER\"; exec sleep 30; fi\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RS_TEST_START_MARKER", marker)
	t.Setenv("ROCKETSCIENCE_MIHOMO", bin)
	s := newTestService(t)
	done := make(chan error, 1)
	go func() { done <- s.Start(direct, freePort(t)) }()
	defer s.Shutdown()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("validation did not begin")
		}
		time.Sleep(10 * time.Millisecond)
	}
	status := make(chan Status, 1)
	go func() { status <- s.Status() }()
	select {
	case st := <-status:
		if !st.Starting || st.Running || st.StartupStage != "校验配置与外部资源" || !st.SystemProxy {
			t.Fatalf("unexpected startup snapshot: %+v", st)
		}
	case <-time.After(time.Second):
		t.Fatal("status blocked behind configuration validation")
	}
	if _, err := s.CheckConnectivity("https://example.com"); err == nil || !strings.Contains(err.Error(), "正在启动") {
		t.Fatalf("network check should report startup promptly: %v", err)
	}
	stop := make(chan error, 1)
	go func() { stop <- s.Stop() }()
	select {
	case err := <-stop:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("stop failed to cancel validation")
	}
	if err := <-done; err == nil || !strings.Contains(err.Error(), "取消") {
		t.Fatalf("expected cancelled start, got %v", err)
	}
	if st := s.Status(); st.Starting || st.Running || st.StartupStage != "" {
		t.Fatalf("startup state leaked after cancellation: %+v", st)
	}
}
