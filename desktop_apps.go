package main

import "rocketscience/internal/network"

// App names the app a connection's process belongs to; Icon is a PNG data URI.
type App struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
}

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
		}
		if app != nil {
			result[path] = *app
		}
	}
	return result
}

var lookupApp = network.AppFor
