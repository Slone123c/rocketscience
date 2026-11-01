//go:build darwin

package main

import "os"

// keepWebKitRenderingInProcess draws the page in its WebContent process.
// A separate com.apple.WebKit.GPU process keeps a blank layer after a force
// quit kills the app process first. The page uses neither canvas, WebGL, nor media.
func keepWebKitRenderingInProcess() {
	os.Args = append(os.Args,
		"--WebKitPreferences.UseGPUProcessForDOMRenderingEnabled=false",
		"--WebKitPreferences.UseGPUProcessForMediaEnabled=false",
	)
}
