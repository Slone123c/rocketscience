//go:build !darwin

package core

import (
	"os"
	"os/exec"
)

type tunLauncher struct {
	cmd         *exec.Cmd
	readyMarker bool
}

func (l *tunLauncher) stop() error { return l.cmd.Process.Signal(os.Interrupt) }
func (*tunLauncher) finished()     {}
