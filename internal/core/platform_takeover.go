//go:build !darwin

package core

func normalizePersistedTakeover(d *profileData) {
	if d.Tun {
		d.Tun = false
	}
	if d.HideDockWhenClosed {
		d.HideDockWhenClosed = false
	}
}

func TunTakeoverAvailable() bool { return false }
