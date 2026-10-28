package core

import (
	"sync"
	"testing"
)

func TestSpeedSettingsLifecycle(t *testing.T) {
	p := NewProfiles(t.TempDir())
	defaults, err := p.SpeedSettings()
	if err != nil || defaults != defaultSpeedSettings() {
		t.Fatalf("defaults: %+v %v", defaults, err)
	}
	if err := p.SetPort(47891); err != nil {
		t.Fatal(err)
	}
	want := SpeedSettings{URL: "https://example.com/check?source=rocket", TimeoutMS: 1200}
	got, err := p.SetSpeedSettings(SpeedSettings{URL: "  " + want.URL + "  ", TimeoutMS: want.TimeoutMS})
	if err != nil || got != want {
		t.Fatalf("save: %+v %v", got, err)
	}
	got, err = NewProfiles(p.dir).SpeedSettings()
	if err != nil || got != want {
		t.Fatalf("reload: %+v %v", got, err)
	}
	state, err := p.State()
	if err != nil || state.MixedPort != 47891 {
		t.Fatalf("other settings lost: %+v %v", state, err)
	}
	for _, bad := range []SpeedSettings{
		{URL: "file:///tmp/test", TimeoutMS: 100}, {URL: "https://user:pass@example.com", TimeoutMS: 100},
		{URL: "https://example.com/#fragment", TimeoutMS: 100}, {URL: "https://example.com", TimeoutMS: 99},
		{URL: "https://example.com", TimeoutMS: 30001},
	} {
		if _, err := p.SetSpeedSettings(bad); err == nil {
			t.Fatalf("accepted invalid setting: %+v", bad)
		}
	}
	got, err = p.SpeedSettings()
	if err != nil || got != want {
		t.Fatalf("invalid save changed settings: %+v %v", got, err)
	}
}

func TestSpeedSettingsConcurrentWithOtherSettings(t *testing.T) {
	p := NewProfiles(t.TempDir())
	var wg sync.WaitGroup
	var speedErr, portErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, speedErr = p.SetSpeedSettings(SpeedSettings{URL: "http://example.com", TimeoutMS: 100})
	}()
	go func() { defer wg.Done(); portErr = p.SetPort(47123) }()
	wg.Wait()
	if speedErr != nil || portErr != nil {
		t.Fatalf("writes: %v %v", speedErr, portErr)
	}
	speed, err := p.SpeedSettings()
	if err != nil || speed.TimeoutMS != 100 {
		t.Fatalf("speed: %+v %v", speed, err)
	}
	state, err := p.State()
	if err != nil || state.MixedPort != 47123 {
		t.Fatalf("port: %+v %v", state, err)
	}
}
