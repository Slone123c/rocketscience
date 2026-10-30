//go:build !windows

package core

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartupValidationReportsProgressAndCanBeCancelled(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", filepath.Join(t.TempDir(), "missing"))
	s := newTestService(t)
	validating := make(chan struct{})
	s.validateRuntime = func(ctx context.Context, dir, path string) error {
		close(validating)
		<-ctx.Done()
		return ctx.Err()
	}
	done := make(chan error, 1)
	go func() { done <- s.Start(direct, freePort(t)) }()
	defer s.Shutdown()
	select {
	case <-validating:
	case <-time.After(3 * time.Second):
		t.Fatal("validation did not begin")
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
