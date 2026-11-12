package systemproxy

// AutomaticAvailable reports whether the OS desktop API can apply proxy settings.
func AutomaticAvailable() bool {
	return automaticAvailable()
}

// ManualFallbackNote is user-facing guidance when AutomaticAvailable is false.
func ManualFallbackNote() string {
	return manualFallbackNote()
}
