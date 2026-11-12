//go:build !windows && !linux

package systemproxy

func nativeJournalChange(Service, int) (Change, bool) { return Change{}, false }
