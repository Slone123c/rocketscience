//go:build darwin

package core

func normalizePersistedTakeover(d *profileData) {}

func TunTakeoverAvailable() bool { return true }
