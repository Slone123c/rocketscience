//go:build !darwin

package network

func TraySpeeds(string, string) ([]byte, bool) { return nil, false }
