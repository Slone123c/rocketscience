//go:build darwin

package systemproxy

func automaticAvailable() bool { return true }

func manualFallbackNote() string { return "" }
