//go:build !darwin

package core

import (
	"os"
	"os/exec"
)

type tunLauncher struct {
	cmd         *exec.Cmd
	readyMarker bool
	exited      chan struct{} // nil on non-macOS; core start uses select/default when TUN helper is unused
}

func (l *tunLauncher) stop() error { return l.cmd.Process.Signal(os.Interrupt) }
func (*tunLauncher) finished()     {}
