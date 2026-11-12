//go:build darwin

package systemproxy

func automaticAvailablePlatform() bool { return true }

func manualFallbackNote() string { return "" }
