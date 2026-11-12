//go:build windows

package systemproxy

func automaticAvailablePlatform() bool { return true }

func manualFallbackNote() string { return "" }
