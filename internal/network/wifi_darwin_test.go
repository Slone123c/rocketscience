//go:build darwin

package network

import (
	"os"
	"testing"
)

func TestWiFiNativeModuleSymbols(t *testing.T) {
	if _, err := os.Stat("../../resources/bin/libRocketNetwork.dylib"); err != nil {
		t.Skip("build the native helpers first")
	}
	if err := load(); err != nil {
		t.Fatal(err)
	}
	if snapshotFn == nil || freeFn == nil || authorizeFn == nil {
		t.Fatal("native network entry point missing")
	}
}
