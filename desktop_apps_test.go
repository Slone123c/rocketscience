package main

import (
	"testing"

	"rocketscience/internal/network"
)

func TestAppsCachesLookupsAndSkipsTools(t *testing.T) {
	calls := 0
	lookupApp = func(path string) (network.App, bool) {
		calls++
		return network.App{Name: "Safari", Icon: "data:image/png;base64,AA"}, path == "/Applications/Safari.app/Contents/MacOS/Safari"
	}
	defer func() { lookupApp = network.AppFor }()
	d := &Desktop{}
	paths := []string{"/Applications/Safari.app/Contents/MacOS/Safari", "/usr/bin/curl"}
	first := d.Apps(paths)
	second := d.Apps(paths)
	if calls != 2 {
		t.Fatalf("lookups = %d, want 2", calls)
	}
	if len(first) != 1 || first[paths[0]].Name != "Safari" || len(second) != 1 {
		t.Fatalf("apps = %v / %v", first, second)
	}
}
