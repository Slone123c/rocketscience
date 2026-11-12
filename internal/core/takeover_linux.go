//go:build linux

package core

import (
	"os"
	"path/filepath"
	"rocketscience/internal/systemproxy"
)

func newProxyController() proxyController {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil
	}
	return systemproxy.New(&systemproxy.LinuxBackend{}, filepath.Join(dir, "RocketScience", "proxy-recovery.json"))
}
