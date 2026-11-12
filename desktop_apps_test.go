package main

import (
	"fmt"
	"testing"
	"time"

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

func TestAppsCacheEvictsOldest(t *testing.T) {
	lookupApp = func(path string) (network.App, bool) {
		return network.App{Name: path, Icon: "data:image/png;base64,AA"}, true
	}
	defer func() { lookupApp = network.AppFor }()
	d := &Desktop{}
	for i := 0; i < appsCacheLimit+5; i++ {
		path := fmt.Sprintf("/Applications/App%d.app/Contents/MacOS/App%d", i, i)
		d.Apps([]string{path})
	}
	if len(d.apps) != appsCacheLimit || len(d.appsOrder) != appsCacheLimit {
		t.Fatalf("cache size apps=%d order=%d", len(d.apps), len(d.appsOrder))
	}
	if _, ok := d.apps["/Applications/App0.app/Contents/MacOS/App0"]; ok {
		t.Fatal("oldest entry was not evicted")
	}
	newest := fmt.Sprintf("/Applications/App%d.app/Contents/MacOS/App%d", appsCacheLimit+4, appsCacheLimit+4)
	if _, ok := d.apps[newest]; !ok {
		t.Fatal("newest entry missing")
	}
}

func TestTrayTickInterval(t *testing.T) {
	if got := trayTickInterval("icon"); got != 5*time.Second {
		t.Fatalf("icon interval %s", got)
	}
	if got := trayTickInterval("both"); got != time.Second {
		t.Fatalf("both interval %s", got)
	}
	if got := trayTickInterval("speed"); got != time.Second {
		t.Fatalf("speed interval %s", got)
	}
}
