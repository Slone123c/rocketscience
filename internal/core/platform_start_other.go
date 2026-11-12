//go:build !darwin

package core

func takeoverRequiredBeforeStart() bool { return false }
