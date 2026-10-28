//go:build darwin

package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNormalCoreSupervisorCrashStopsCore(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO")
	}
	s := New()
	proxy := &fakeProxy{}
	s.proxy = proxy
	s.geo.dir = t.TempDir()
	defer s.Shutdown()
	if err := s.SetSystemProxy(true); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(direct, freePort(t)); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	dir, supervisor, done := s.runtimeDir, s.cmd, s.done
	s.mu.Unlock()
	raw, err := os.ReadFile(filepath.Join(dir, "core.pid"))
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || !processAlive(pid) {
		t.Fatalf("core PID %q %v", raw, err)
	}
	if err = supervisor.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("supervisor crash cleanup timed out")
	}
	if processAlive(pid) {
		t.Fatal("core survived supervisor crash")
	}
	pending := func() bool { s.mu.Lock(); defer s.mu.Unlock(); return proxy.pending }
	deadline := time.Now().Add(5 * time.Second)
	for pending() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if pending() {
		t.Fatal("system proxy was not restored after core stopped")
	}
}
func TestNormalOrphanCleanupRejectsUnrelatedProcess(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("/bin/sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	if err := os.WriteFile(filepath.Join(dir, "core.pid"), []byte(strconv.Itoa(cmd.Process.Pid)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := cleanupOrdinaryOrphan(dir, "/bin/sleep"); err == nil {
		t.Fatal("accepted unrelated process")
	}
	if !processAlive(cmd.Process.Pid) {
		t.Fatal("unrelated process was stopped")
	}
}
