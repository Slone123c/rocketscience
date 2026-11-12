package main

import (
	"errors"

	"github.com/egoist/mygo"
)

// dockShouldShow decides whether the Dock icon stays while the dashboard is closed.
// Minimized windows keep a Dock tile, so they still count as shown.
func dockShouldShow(hideWhenClosed, hasMain, visible, minimized bool) bool {
	if !hideWhenClosed {
		return true
	}
	if !hasMain {
		return false
	}
	return visible || minimized
}

func (d *Desktop) HideDockWhenClosed() (bool, error) {
	if d.profiles == nil {
		return false, errors.New("设置不可用")
	}
	return d.profiles.HideDockWhenClosed()
}

func (d *Desktop) SetHideDockWhenClosed(enabled bool) error {
	if d.profiles == nil {
		return errors.New("设置不可用")
	}
	if err := d.profiles.SetHideDockWhenClosed(enabled); err != nil {
		return err
	}
	d.syncDockVisibility()
	return nil
}

func (d *Desktop) showDock() {
	if d.dockShow != nil {
		d.dockShow()
		return
	}
	mygo.App.Dock.Show()
}

func (d *Desktop) hideDock() {
	if d.dockHide != nil {
		d.dockHide()
		return
	}
	mygo.App.Dock.Hide()
}

// syncDockVisibility keeps the Dock icon aligned with the dashboard and the preference.
func (d *Desktop) syncDockVisibility() {
	hideWhenClosed := false
	if d.profiles != nil {
		if value, err := d.profiles.HideDockWhenClosed(); err == nil {
			hideWhenClosed = value
		}
	}
	d.mu.Lock()
	main := d.main
	d.mu.Unlock()
	hasMain, visible, minimized := false, false, false
	if main != nil {
		hasMain = true
		visible = main.IsVisible()
		minimized = main.IsMinimized()
	}
	if dockShouldShow(hideWhenClosed, hasMain, visible, minimized) {
		d.showDock()
		return
	}
	d.hideDock()
}
