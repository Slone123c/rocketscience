package systemproxy

// automaticAvailableOverride lets tests supply availability without faking the whole desktop session.
// The bool is (available, ok); ok=false falls through to the platform probe.
var automaticAvailableOverride func() (bool, bool)

func SetAutomaticAvailableOverride(fn func() (bool, bool)) {
	automaticAvailableOverride = fn
}

// AutomaticAvailable reports whether the OS desktop API can apply proxy settings.
func AutomaticAvailable() bool {
	if automaticAvailableOverride != nil {
		if v, ok := automaticAvailableOverride(); ok {
			return v
		}
	}
	return automaticAvailablePlatform()
}

// ManualFallbackNote is user-facing guidance when AutomaticAvailable is false.
func ManualFallbackNote() string {
	return manualFallbackNote()
}
