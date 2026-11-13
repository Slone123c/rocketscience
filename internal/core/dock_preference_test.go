package core

import (
	"runtime"
	"testing"
)

func TestHideDockWhenClosedPreference(t *testing.T) {
	p := NewProfiles(t.TempDir())
	value, err := p.HideDockWhenClosed()
	if err != nil || value {
		t.Fatalf("default %v %v", value, err)
	}
	if err := p.SetHideDockWhenClosed(true); err != nil {
		t.Fatal(err)
	}
	value, err = NewProfiles(p.dir).HideDockWhenClosed()
	want := runtime.GOOS == "darwin"
	if err != nil || value != want {
		t.Fatalf("saved: got %v want %v (err=%v)", value, want, err)
	}
	if err := p.SetHideDockWhenClosed(false); err != nil {
		t.Fatal(err)
	}
	value, err = NewProfiles(p.dir).HideDockWhenClosed()
	if err != nil || value {
		t.Fatalf("cleared %v %v", value, err)
	}
}
