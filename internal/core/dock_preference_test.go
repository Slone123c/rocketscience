package core

import "testing"

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
	if err != nil || !value {
		t.Fatalf("saved %v %v", value, err)
	}
	if err := p.SetHideDockWhenClosed(false); err != nil {
		t.Fatal(err)
	}
	value, err = NewProfiles(p.dir).HideDockWhenClosed()
	if err != nil || value {
		t.Fatalf("cleared %v %v", value, err)
	}
}
