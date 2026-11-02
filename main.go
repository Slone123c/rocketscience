package main

import (
	_ "embed"
	"github.com/egoist/mygo"
	"log"
	"os"
	"path/filepath"
	"rocketscience/internal/core"
	"rocketscience/internal/network"
	"runtime/debug"
)

//go:embed resources/icon.png
var appIcon []byte

func hasBundledIcon() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(filepath.Dir(exe), "..", "Resources", "AppIcon.icns"))
	return err == nil
}

func main() {
	// A proxy mostly allocates short-lived buffers; collect them earlier to keep the heap small.
	debug.SetGCPercent(50)
	mygo.App.SetName("RocketScience")
	keepWebKitRenderingInProcess()
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	service := core.New()
	mygo.Bind(service)
	profiles := core.NewProfilesWithService(os.Getenv("ROCKETSCIENCE_DATA_DIR"), service)
	desktop := &Desktop{service: service, profiles: profiles}
	mygo.Bind(desktop)
	mygo.Bind(profiles)
	wifi := core.NewWiFi(profiles, service, func() (network.Snapshot, error) {
		var snapshot network.Snapshot
		var err error
		mygo.RunOnMain(func() { snapshot, err = network.Read() })
		return snapshot, err
	})
	mygo.Bind(wifi)
	stopUpdates := make(chan struct{})
	mygo.App.WhenReady(func() {
		// The packaged app already has AppIcon.icns. Setting the PNG as well
		// keeps a 32 MB rendering of it in memory, so only unpackaged runs do.
		if !hasBundledIcon() {
			if err := mygo.App.Dock.SetIcon(appIcon); err != nil {
				log.Printf("无法加载应用图标：%v", err)
			}
		}
		go core.RunUpdates(profiles, stopUpdates)
		desktop.ready()
		go func() {
			resources, err := mygo.App.Path(mygo.PathResources)
			if err == nil {
				err = service.InstallBundledGeo(resources)
			}
			if err != nil {
				log.Printf("无法安装随包 GEO 数据：%v", err)
			}
			_ = service.RecoverGeoCache()
			err = profiles.Initialize()
			_ = wifi.Check()
			desktop.completeStartup(err)
			core.RunWiFi(wifi, stopUpdates)
		}()
	})
	mygo.App.OnBeforeQuit(desktop.beforeQuit)
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.OnActivate(func(_ bool) {
		// A tray popover can be visible while the dashboard is hidden.
		// Reopening the app must restore the dashboard in either case.
		desktop.OpenDashboard()
	})
	mygo.App.OnQuit(func() { close(stopUpdates); desktop.destroy(); service.Shutdown() })
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
