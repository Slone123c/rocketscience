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

func TestParseUsageDecimalScientificAndCaseInsensitive(t *testing.T) {
	u := parseUsage("total=1125899906842624; DOWNLOAD=123.0; Upload=1.5e9")
	if u == nil {
		t.Fatal("parseUsage returned nil")
	}
	if u.Upload != 1500000000 {
		t.Fatalf("upload: got %d, want 1500000000", u.Upload)
	}
	if u.Download != 123 {
		t.Fatalf("download: got %d, want 123", u.Download)
	}
	if u.Total != 1125899906842624 {
		t.Fatalf("total: got %d, want 1125899906842624", u.Total)
	}
	if u.Expire != 0 {
		t.Fatalf("expire: got %d, want 0", u.Expire)
	}
	withExpire := parseUsage("expire=2000000000.0; upload=-5")
	if withExpire == nil || withExpire.Expire != 2000000000 || withExpire.Upload != 0 {
		t.Fatalf("withExpire: %+v", withExpire)
	}
	if parseUsage("upload=-1; download=NaN; total=Inf") != nil {
		t.Fatal("expected nil for all-invalid usage")
	}
}

func TestConcurrentManualAndScheduledUpdateDownloadsOnce(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO to test a real core")
	}
	inFlight := make(chan struct{}, 2)
	release := make(chan struct{})
	var downloads int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloads++
		inFlight <- struct{}{}
		<-release
		_, _ = w.Write([]byte(direct))
	}))
	defer server.Close()
	p := NewProfiles(t.TempDir())
	state, err := p.Save(Profile{Name: "sub", Source: "url", URL: server.URL, Content: direct, Interval: 6, LastAttempt: time.Now().Add(-7 * time.Hour)}, "")
	if err != nil {
		t.Fatal(err)
	}
	id := state.ActiveID
	manualDone := make(chan error, 1)
	go func() {
		_, err := p.Update(id)
		manualDone <- err
	}()
	<-inFlight
	schedDone := make(chan error, 1)
	go func() {
		_, err := p.updateSubscription(id, true)
		schedDone <- err
	}()
	time.Sleep(30 * time.Millisecond)
	close(release)
	if err := <-manualDone; err != nil {
		t.Fatalf("manual update failed: %v", err)
	}
	if err := <-schedDone; err != nil {
		t.Fatalf("scheduled update failed: %v", err)
	}
	if downloads != 1 {
		t.Fatalf("expected exactly 1 download when manual and scheduled update overlap, got %d", downloads)
	}
}

func TestDeleteAndUseMaintainActiveProfileIntegrity(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("set ROCKETSCIENCE_MIHOMO to test a real core")
	}
	s := newTestService(t)
	defer s.Shutdown()
	p := NewProfilesWithService(t.TempDir(), s)
	if err := p.SetPort(freePort(t)); err != nil {
		t.Fatal(err)
	}
	st1, err := p.Save(Profile{Name: "P1", Source: "file", Content: direct}, "")
	if err != nil {
		t.Fatal(err)
	}
	id1 := st1.ActiveID
	rejectCfg := strings.Replace(direct, "MATCH,DIRECT", "MATCH,REJECT", 1)
	st2, err := p.Save(Profile{Name: "P2", Source: "file", Content: rejectCfg}, "")
	if err != nil {
		t.Fatal(err)
	}
	id2 := st2.Profiles[1].ID
	if err := p.StartActive(); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Delete(id1); err == nil {
		t.Fatal("deleted active profile while running")
	}
	if _, err := p.Use(id2); err != nil {
		t.Fatal(err)
	}
	rules, err := s.Rules()
	if err != nil || len(rules) != 1 || rules[0].Proxy != "REJECT" {
		t.Fatalf("Use(P2) did not switch running core rules: %+v %v", rules, err)
	}
	afterDelete, err := p.Delete(id1)
	if err != nil || len(afterDelete.Profiles) != 1 || afterDelete.ActiveID != id2 {
		t.Fatalf("Delete(P1) after switching to P2 corrupted state: %+v %v", afterDelete, err)
	}
}
