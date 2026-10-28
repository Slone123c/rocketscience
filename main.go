package main

import (
	"github.com/egoist/mygo"
	"log"
	"os"
	"rocketscience/internal/core"
	"rocketscience/internal/network"
)

func main() {
	mygo.App.SetName("RocketScience")
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
		go core.RunUpdates(profiles, stopUpdates)
		desktop.ready()
		go func() {
			_ = service.RecoverGeoCache()
			err := profiles.Initialize()
			_ = wifi.Check()
			desktop.completeStartup(err)
			core.RunWiFi(wifi, stopUpdates)
		}()
	})
	mygo.App.OnBeforeQuit(desktop.beforeQuit)
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.OnActivate(func(visible bool) {
		if !visible {
			desktop.OpenDashboard()
		}
	})
	mygo.App.OnQuit(func() { close(stopUpdates); desktop.destroy(); service.Shutdown() })
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
