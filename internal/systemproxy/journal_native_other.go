//go:build !windows && !linux

package systemproxy

func nativeJournalChanges(Service, int, *[]string) []Change { return nil }
