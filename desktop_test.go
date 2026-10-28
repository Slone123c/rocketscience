package main

import (
	"errors"
	"testing"
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
