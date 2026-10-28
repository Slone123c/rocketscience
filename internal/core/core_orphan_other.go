//go:build !darwin

package core

func cleanupOrdinaryOrphan(string, string) error { return nil }
