package main

import "rocketscience/internal/network"

// App names the app a connection's process belongs to; Icon is a PNG data URI.
type App struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// appsCacheLimit caps remembered icons so the connections page cannot grow the
// process forever while browsing many paths.
const appsCacheLimit = 64

// Apps looks up the app around each executable path. Command-line tools are
// left out, so the connections page draws a terminal tile for them.
func (d *Desktop) Apps(paths []string) map[string]App {
	d.appsMu.Lock()
	defer d.appsMu.Unlock()
	if d.apps == nil {
		d.apps = map[string]*App{}
	}
	result := map[string]App{}
	for _, path := range paths {
		app, seen := d.apps[path]
		if !seen {
			if found, ok := lookupApp(path); ok {
				app = &App{Name: found.Name, Icon: found.Icon}
			}
			d.apps[path] = app
			d.appsOrder = append(d.appsOrder, path)
			d.trimAppsCacheLocked()
		} else {
			d.touchAppsCacheLocked(path)
		}
		if app != nil {
			result[path] = *app
		}
	}
	return result
}

func (d *Desktop) touchAppsCacheLocked(path string) {
	for i, key := range d.appsOrder {
		if key != path {
			continue
		}
		d.appsOrder = append(append([]string{}, d.appsOrder[:i]...), d.appsOrder[i+1:]...)
		d.appsOrder = append(d.appsOrder, path)
		return
	}
}

func (d *Desktop) trimAppsCacheLocked() {
	for len(d.appsOrder) > appsCacheLimit {
		oldest := d.appsOrder[0]
		d.appsOrder = d.appsOrder[1:]
		delete(d.apps, oldest)
	}
}

var lookupApp = network.AppFor
