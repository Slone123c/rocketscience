//go:build !windows && !linux

package systemproxy

func nativeJournalChange(Service, int, *[]string) (Change, bool) { return Change{}, false }
