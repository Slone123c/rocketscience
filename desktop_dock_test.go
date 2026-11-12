package main

import (
	"rocketscience/internal/core"
	"testing"
)

func TestDockShouldShow(t *testing.T) {
	cases := []struct {
		name                                          string
		hideWhenClosed, hasMain, visible, minimized bool
		want                                          bool
	}{
		{name: "preference off", hideWhenClosed: false, hasMain: false, want: true},
		{name: "closed dashboard", hideWhenClosed: true, hasMain: true, visible: false, want: false},
		{name: "visible dashboard", hideWhenClosed: true, hasMain: true, visible: true, want: true},
		{name: "minimized dashboard", hideWhenClosed: true, hasMain: true, minimized: true, want: true},
		{name: "released page", hideWhenClosed: true, hasMain: false, want: false},
	}
	for _, tc := range cases {
		if got := dockShouldShow(tc.hideWhenClosed, tc.hasMain, tc.visible, tc.minimized); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestHideDockWhenClosedSettingBridge(t *testing.T) {
	p := core.NewProfiles(t.TempDir())
	shown, hidden := 0, 0
	d := &Desktop{
		profiles: p,
		dockShow: func() { shown++ },
		dockHide: func() { hidden++ },
	}
	if value, err := d.HideDockWhenClosed(); err != nil || value {
		t.Fatalf("default: %v %v", value, err)
	}
	if err := d.SetHideDockWhenClosed(true); err != nil {
		t.Fatal(err)
	}
	if value, err := d.HideDockWhenClosed(); err != nil || !value {
		t.Fatalf("saved: %v %v", value, err)
	}
	if hidden != 1 || shown != 0 {
		t.Fatalf("closed dashboard should hide Dock: shown=%d hidden=%d", shown, hidden)
	}
	if err := d.SetHideDockWhenClosed(false); err != nil {
		t.Fatal(err)
	}
	if shown != 1 {
		t.Fatalf("turning preference off should show Dock: shown=%d", shown)
	}
}
