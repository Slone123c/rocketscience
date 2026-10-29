//go:build !darwin

package core

import (
	"errors"
	"os"
	"os/exec"
)

type tunLauncher struct {
	cmd         *exec.Cmd
	readyMarker bool
}

func launchTun(string, string) (*tunLauncher, error) {
	return nil, errors.New("TUN 目前仅支持 macOS")
}
func launchUnprivileged(core, dir string) (*tunLauncher, error) {
	cmd := exec.Command(core, "-d", dir, "-f", dir+"/config.yaml")
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &tunLauncher{cmd: cmd}, nil
}
func (l *tunLauncher) stop() error { return l.cmd.Process.Signal(os.Interrupt) }
func (*tunLauncher) finished()     {}
