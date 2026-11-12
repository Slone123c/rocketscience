//go:build windows

package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenGeoSourceRejectsReparsePoint(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "country.mmdb")
	if err := os.WriteFile(target, []byte("safe"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "country-link.mmdb")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("需要创建符号链接的权限")
	}
	f, err := openGeoSourceFile(link)
	if err == nil {
		f.Close()
		t.Fatal("expected symlink to be rejected")
	}
	data, err := openGeoSourceFile(target)
	if err != nil {
		t.Fatalf("regular file: %v", err)
	}
	_ = data.Close()
}

func TestValidGeoRuntimeUsesSecurityNotModePerm(t *testing.T) {
	dir, err := os.MkdirTemp("", "rocketscience-core-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := validGeoRuntime(dir); err != nil {
		t.Fatalf("MkdirTemp runtime rejected: %v", err)
	}
	bad := filepath.Join(t.TempDir(), "rocketscience-core-outside")
	if err := os.Mkdir(bad, 0777); err != nil {
		t.Fatal(err)
	}
	if err := validGeoRuntime(bad); err == nil {
		t.Fatal("expected location check failure")
	}
}

func TestProcessAliveUsesExitCode(t *testing.T) {
	if processAlive(1<<30) {
		t.Fatal("nonexistent pid should not be alive")
	}
	cmd := exec.Command("cmd", "/C", "exit")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	deadline := time.Now().Add(5 * time.Second)
	for processAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if processAlive(pid) {
		t.Fatal("exited process still reported alive")
	}
}
