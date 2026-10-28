//go:build !darwin

package core

func newProxyController() proxyController { return nil }
