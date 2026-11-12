package systemproxy

// platformNativeJournalChange records OS-native proxy fields needed for lossless restore after restart.
func platformNativeJournalChange(s Service, port int) (Change, bool) {
	return nativeJournalChange(s, port)
}
