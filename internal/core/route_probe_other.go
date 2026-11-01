//go:build !darwin

package core

func traceForward(string) ForwardRoute { return ForwardRoute{Error: "当前平台不检测去程"} }
