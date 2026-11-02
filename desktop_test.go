package main

import (
	"errors"
	"testing"
	"time"
)

func TestStartupDesktopStateAndLoginAdapter(t *testing.T) {
	enabled := false
	d := &Desktop{loginRead: func() bool { return enabled }, loginWrite: func(value bool) error {
		if value {
			return errors.New("approval required")
		}
		enabled = false
		return nil
	}}
	if d.State().StartupReady {
		t.Fatal("startup should begin pending")
	}
	d.completeStartup(errors.New("automatic connection failed"))
	if state := d.State(); !state.StartupReady || state.StartupError != "automatic connection failed" {
		t.Fatalf("state: %+v", state)
	}
	if err := d.SetOpenAtLogin(true); err == nil || d.OpenAtLogin() {
		t.Fatal("approval failure ignored")
	}
	enabled = true
	if err := d.SetOpenAtLogin(false); err != nil || d.OpenAtLogin() {
		t.Fatal("login item did not disable")
	}
}

func TestPageIdleReleasesOnlyAfterStayingHidden(t *testing.T) {
	var idle pageIdle
	start := time.Unix(1000, 0)
	if idle.expired(true, false, start) {
		t.Fatal("released as soon as it was hidden")
	}
	if idle.expired(true, false, start.Add(pageIdleRelease-time.Second)) {
		t.Fatal("released before the grace period")
	}
	if !idle.expired(true, false, start.Add(pageIdleRelease)) {
		t.Fatal("not released after the grace period")
	}
	if idle.expired(false, false, start.Add(2*pageIdleRelease)) {
		t.Fatal("released while visible")
	}
	if idle.expired(true, false, start.Add(3*pageIdleRelease)) {
		t.Fatal("showing the window did not restart the grace period")
	}
	if idle.expired(true, true, start.Add(5*pageIdleRelease)) {
		t.Fatal("released while busy")
	}
}
