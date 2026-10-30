//go:build darwin

package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type tunLauncher struct {
	cmd         *exec.Cmd
	dir         string
	done        chan struct{}
	exited      chan struct{}
	once        sync.Once
	authorizing bool
	readyMarker bool
}

func (l *tunLauncher) heartbeat() {
	defer close(l.exited)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-l.done:
			return
		case <-ticker.C:
			if err := os.WriteFile(filepath.Join(l.dir, "heartbeat"), []byte("running\n"), 0600); err != nil {
				return
			}
		}
	}
}
func (l *tunLauncher) stop() error {
	l.once.Do(func() { close(l.done) })
	<-l.exited
	if err := os.WriteFile(filepath.Join(l.dir, "heartbeat"), []byte("stop\n"), 0600); err != nil {
		return fmt.Errorf("无法通知 TUN 停止：%w", err)
	}
	if l.authorizing && !watchdogHasCorePID(l.dir) && l.cmd != nil && l.cmd.Process != nil {
		// Cancel the unprivileged authorization wrapper. If elevation races this
		// check, the supervisor sees the stop marker before launching mihomo.
		_ = l.cmd.Process.Kill()
	}
	return nil
}
func (l *tunLauncher) finished() { l.once.Do(func() { close(l.done) }); <-l.exited }
