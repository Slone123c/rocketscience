package core

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSubscriptionUpdateAndLiveReload(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO to test a real core")
	}
	response := direct
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("subscription-userinfo", "upload=10; download=20; total=100; expire=2000000000")
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()
	s := newTestService(t)
	defer s.Shutdown()
	p := NewProfilesWithService(t.TempDir(), s)
	state, err := p.Save(Profile{Name: "test", Source: "url", URL: server.URL, Content: direct, Interval: 6}, "")
	if err != nil {
		t.Fatal(err)
	}
	id := state.ActiveID
	if err = p.SetPort(freePort(t)); err != nil {
		t.Fatal(err)
	}
	if err = p.StartActive(); err != nil {
		t.Fatal(err)
	}
	if err = s.SetMode("global"); err != nil {
		t.Fatal(err)
	}
	response = strings.Replace(direct, "MATCH,DIRECT", "MATCH,REJECT", 1)
	state, err = p.Update(id)
	if err != nil {
		t.Fatal(err)
	}
	if state.Profiles[0].Content != response || state.Profiles[0].Usage == nil || state.Profiles[0].Usage.Download != 20 {
		t.Fatalf("update not saved: %+v", state.Profiles[0])
	}
	rules, err := s.Rules()
	if err != nil || len(rules) != 1 || rules[0].Proxy != "REJECT" {
		t.Fatalf("rules not reloaded: %v %v", rules, err)
	}
	if s.Status().Mode != "global" {
		t.Fatal("mode changed during reload")
	}
	old := state.Profiles[0].Content
	response = "rules: ["
	if _, err = p.Update(id); err == nil {
		t.Fatal("invalid subscription accepted")
	}
	state, err = p.State()
	if err != nil || state.Profiles[0].Content != old || state.Profiles[0].LastError == "" {
		t.Fatal("invalid update damaged saved profile")
	}
	rules, err = s.Rules()
	if err != nil || rules[0].Proxy != "REJECT" {
		t.Fatal("invalid update changed running rules")
	}
}
func TestSubscriptionSchedule(t *testing.T) {
	now := time.Now()
	p := Profile{Source: "url", Interval: 6, LastAttempt: now.Add(-5 * time.Hour)}
	if subscriptionDue(p, now) {
		t.Fatal("updated too early")
	}
	if !subscriptionDue(p, now.Add(time.Hour)) {
		t.Fatal("missing due update")
	}
	p.Interval = 0
	if subscriptionDue(p, now.Add(100*time.Hour)) {
		t.Fatal("disabled subscription should not update")
	}
}

func TestSubscriptionUpdateDoesNotOverwriteConcurrentEdit(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO to test a real core")
	}
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		_, _ = w.Write([]byte(direct))
	}))
	defer server.Close()
	p := NewProfiles(t.TempDir())
	state, err := p.Save(Profile{Name: "test", Source: "url", URL: server.URL, Content: direct, Interval: 6}, "")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, e := p.Update(state.ActiveID); done <- e }()
	<-started
	edited := state.Profiles[0]
	edited.Content = strings.Replace(direct, "MATCH,DIRECT", "MATCH,REJECT", 1)
	if _, err = p.Save(edited, edited.UpdatedAt.Format(time.RFC3339Nano)); err != nil {
		close(release)
		t.Fatal(err)
	}
	close(release)
	if err = <-done; err == nil {
		t.Fatal("concurrent edit was overwritten")
	}
	after, err := p.State()
	if err != nil || after.Profiles[0].Content != edited.Content {
		t.Fatalf("edit not preserved: %v", err)
	}
}

func TestStaleEditorCannotOverwriteUpdatedSubscription(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO to test a real core")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(direct)) }))
	defer server.Close()
	p := NewProfiles(t.TempDir())
	state, err := p.Save(Profile{Name: "test", Source: "url", URL: server.URL, Content: direct}, "")
	if err != nil {
		t.Fatal(err)
	}
	stale := state.Profiles[0]
	if _, err = p.Update(stale.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Save(stale, stale.UpdatedAt.Format(time.RFC3339Nano)); err == nil {
		t.Fatal("stale editor overwrote updated subscription")
	}
}

func TestSchedulerRunsDueSubscription(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO to test a real core")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(direct)) }))
	defer server.Close()
	p := NewProfiles(t.TempDir())
	_, err := p.Save(Profile{Name: "test", Source: "url", URL: server.URL, Content: direct, Interval: 6, LastAttempt: time.Now().Add(-7 * time.Hour)}, "")
	if err != nil {
		t.Fatal(err)
	}
	stop, done := make(chan struct{}), make(chan struct{})
	go func() { RunUpdates(p, stop); close(done) }()
	deadline := time.Now().Add(5 * time.Second)
	updated := false
	for time.Now().Before(deadline) {
		state, e := p.State()
		if e != nil {
			t.Fatal(e)
		}
		if time.Since(state.Profiles[0].LastAttempt) < time.Minute {
			updated = true
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	close(stop)
	<-done
	if !updated {
		t.Fatal("due subscription was not updated")
	}
}
