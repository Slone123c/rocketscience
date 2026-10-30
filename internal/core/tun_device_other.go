//go:build !darwin

package core

import (
	"context"
	"errors"

	"rocketscience/internal/tunhelper"
)

type tunDevice struct{ name string }

func openTunDevice(context.Context, tunhelper.Request) (*tunDevice, error) {
	return nil, errors.New("TUN 仅支持 macOS")
}
func (*tunDevice) dup() (int, error) { return -1, errors.New("TUN 仅支持 macOS") }
func (*tunDevice) Close()            {}
func tunServiceInstalled() bool      { return false }
func uninstallTunService() error     { return nil }
