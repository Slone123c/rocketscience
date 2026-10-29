package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestCancelPendingTunAuthorization(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "core.pid"), []byte("running\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	l := &tunLauncher{cmd: cmd, dir: dir, done: make(chan struct{}), exited: make(chan struct{}), authorizing: true}
	go l.heartbeat()
	if err := l.stop(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("authorization wrapper remained pending after cancel")
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "heartbeat"))
	if string(raw) != "stop\n" {
		t.Fatal("missing stop marker")
	}
}
