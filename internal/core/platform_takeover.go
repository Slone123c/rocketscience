//go:build !darwin

package core

import "rocketscience/internal/systemproxy"

func normalizePersistedTakeover(d *profileData) {
	if d.Tun {
		d.Tun = false
	}
	if d.HideDockWhenClosed {
		d.HideDockWhenClosed = false
	}
	if d.SystemProxy && !systemproxy.AutomaticAvailable() {
		d.SystemProxy = false
	}
}

func TunTakeoverAvailable() bool { return false }
