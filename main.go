package main

import (
	"github.com/egoist/mygo"
	"log"
	"os"
	"rocketscience/internal/core"
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
	stopUpdates := make(chan struct{})
	mygo.App.WhenReady(func() {
		go core.RunUpdates(profiles, stopUpdates)
		desktop.ready()
		go func() { _ = service.RecoverGeoCache(); desktop.completeStartup(profiles.Initialize()) }()
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
