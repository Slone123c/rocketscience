package systemproxy

// appendNativeJournalChanges records OS-native proxy fields needed for lossless restore after restart.
func appendNativeJournalChanges(j *journal, s Service, port int, bypass *[]string) {
	for _, c := range nativeJournalChanges(s, port, bypass) {
		j.Changes = append(j.Changes, c)
	}
}
