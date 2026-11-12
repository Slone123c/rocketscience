//go:build windows

package systemproxy

func automaticAvailable() bool { return true }

func manualFallbackNote() string { return "" }
