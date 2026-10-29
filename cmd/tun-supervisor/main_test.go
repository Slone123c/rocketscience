package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func fixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"config.yaml", "heartbeat", "tun-core.log", "core.pid", "core.ready"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("running"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	core := filepath.Join(dir, "core")
	if err := os.WriteFile(core, []byte("#!/bin/sh\nexec /bin/sleep 30\n"), 0700); err != nil {
		t.Fatal(err)
	}
	return dir, core
}

func TestCoreReadyCommandQuotesPrivatePath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "space ' and $(echo unused)")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("/bin/sh", "-c", coreReadyCommand(dir)).Run(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "core.ready"))
	if err != nil || string(data) != "ready" {
		t.Fatalf("missing ready marker: %v", err)
	}
}
func TestSupervisorStopsWithOwner(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("requires non-root owner")
	}
	dir, core := fixture(t)
	finished := make(chan error, 1)
	go func() { finished <- run([]string{core, dir, strconv.Itoa(os.Getuid())}) }()
	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "heartbeat"), []byte("stop"), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("supervisor failed to stop child")
	}
}
func TestSupervisorRejectsUnsafeDirectory(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("requires non-root owner")
	}
	dir, core := fixture(t)
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{core, dir, strconv.Itoa(os.Getuid())}); err == nil {
		t.Fatal("accepted public runtime directory")
	}
}
func TestSupervisorExitsOnStaleHeartbeat(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("requires non-root owner")
	}
	dir, core := fixture(t)
	old := time.Now().Add(-15 * time.Second)
	if err := os.Chtimes(filepath.Join(dir, "heartbeat"), old, old); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() { finished <- run([]string{core, dir, strconv.Itoa(os.Getuid())}) }()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("stale heartbeat left child running")
	}
}

func TestCancelledAuthorizationNeverLaunchesCore(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("requires non-root owner")
	}
	dir, core := fixture(t)
	if err := os.WriteFile(filepath.Join(dir, "heartbeat"), []byte("stop\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{core, dir, strconv.Itoa(os.Getuid())}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "core.pid"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "running" {
		t.Fatal("cancelled authorization launched a core")
	}
}
