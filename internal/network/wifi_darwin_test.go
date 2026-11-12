//go:build darwin

package network

import (
	"os"
	"testing"
)

func TestWiFiNativeModuleSymbols(t *testing.T) {
	if _, err := os.Stat("../../resources/darwin/bin/libRocketNetwork.dylib"); err != nil {
		t.Skip("build the native helpers first")
	}
	if err := load(); err != nil {
		t.Fatal(err)
	}
	if snapshotFn == nil || freeFn == nil || authorizeFn == nil || appIconFn == nil {
		t.Fatal("native network entry point missing")
	}
}

func TestAppForFindsEnclosingApp(t *testing.T) {
	if _, err := os.Stat("../../resources/darwin/bin/libRocketNetwork.dylib"); err != nil {
		t.Skip("build the native helpers first")
	}
	app, ok := AppFor("/System/Applications/Calculator.app/Contents/MacOS/Calculator")
	if !ok || app.Name == "" || len(app.Icon) < 100 {
		t.Fatalf("app = %q ok = %v", app.Name, ok)
	}
	if _, ok := AppFor("/usr/bin/curl"); ok {
		t.Fatal("command-line tool reported an app")
	}
}
