//go:build !darwin

package network

func AppFor(string) (App, bool) { return App{}, false }
