//go:build !darwin && !windows && !linux

package core

func newProxyController() proxyController { return nil }
