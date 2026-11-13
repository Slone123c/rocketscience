package core

import (
	"errors"
	"rocketscience/internal/systemproxy"
	"testing"
	"time"
)

type fakeProxy struct {
	pending    bool
	applyErr   error
	restoreErr error
	applied    int
	bypass     *[]string
	restored   int
}

func (p *fakeProxy) ApplyWithBypass(port int, bypass *[]string) error {
	p.applied = port
	p.bypass = cloneBypass(bypass)
	p.pending = true
	return p.applyErr
}
func (p *fakeProxy) Restore() ([]string, error) {
	p.restored++
	if p.restoreErr != nil {
		return nil, p.restoreErr
	}
	p.pending = false
	return nil, nil
}
func (p *fakeProxy) Pending() (bool, error) { return p.pending, nil }
func TestTakeoverRestoresBeforeCoreStops(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", t.TempDir()+"/missing")
	p := &fakeProxy{}
	withAutomaticSystemProxy(t)
	s := New()
	s.proxy = p
	defer s.Shutdown()
	if err := s.SetSystemProxy(true); err != nil {
		t.Fatal(err)
	}
	port := freePort(t)
	if err := s.Start(direct, port); err != nil {
		t.Fatal(err)
	}
	if p.applied != port || !p.pending {
		t.Fatal("proxy not applied")
	}
	p.restoreErr = errors.New("permission denied")
	if err := s.Stop(); err == nil || !s.Status().Running {
		t.Fatal("core stopped without restoring network")
	}
	p.restoreErr = nil
	if err := s.Stop(); err != nil || s.Status().Running || p.pending {
		t.Fatalf("stop did not restore: %v", err)
	}
}
func TestPendingRecoveryBlocksStart(t *testing.T) {
	p := &fakeProxy{pending: true}
	withAutomaticSystemProxy(t)
	s := New()
	s.proxy = p
	if err := s.SetSystemProxy(true); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(direct, 7890); err == nil {
		t.Fatal("start should require recovery")
	}
	if err := s.RecoverNetwork(); err != nil || p.pending {
		t.Fatalf("recovery failed: %v", err)
	}
}

func TestStartRequiresTakeover(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", t.TempDir()+"/missing")
	withAutomaticSystemProxy(t)
	s := New()
	s.proxy = &fakeProxy{}
	s.geo.dir = testGeoDir(t)
	defer s.Shutdown()
	err := s.Start(direct, freePort(t))
	if takeoverRequiredBeforeStart() {
		if err == nil {
			t.Fatal("core started without takeover")
		}
		return
	}
	if err != nil {
		t.Fatalf("manual mode start on non-macOS failed: %v", err)
	}
}

func TestDisablingLastTakeoverStopsCore(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", t.TempDir()+"/missing")
	p := &fakeProxy{}
	withAutomaticSystemProxy(t)
	s := New()
	s.proxy = p
	defer s.Shutdown()
	if err := s.SetSystemProxy(true); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(direct, freePort(t)); err != nil {
		t.Fatal(err)
	}
	p.restoreErr = errors.New("permission denied")
	if err := s.SetSystemProxy(false); err == nil {
		t.Fatal("restore failure ignored")
	}
	if state := s.Status(); !state.Running || !state.SystemProxy {
		t.Fatal("failed restore changed active state")
	}
	p.restoreErr = nil
	if err := s.SetSystemProxy(false); err != nil {
		t.Fatal(err)
	}
	if state := s.Status(); state.Running || state.SystemProxy || p.pending {
		t.Fatal("last takeover was not stopped and restored")
	}
}
func TestFailureToApplyRollsBack(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", t.TempDir()+"/missing")
	p := &fakeProxy{applyErr: errors.Join(systemproxy.ErrRecoveryNeeded, errors.New("permission denied"))}
	withAutomaticSystemProxy(t)
	s := New()
	s.proxy = p
	defer s.Shutdown()
	if err := s.SetSystemProxy(true); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(direct, freePort(t)); err == nil {
		t.Fatal("failed takeover succeeded")
	}
	if p.pending || p.restored != 1 || s.Status().Running {
		t.Fatal("failed takeover did not roll back")
	}
}

func TestCrashRestoresTakeover(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", t.TempDir()+"/missing")
	p := &fakeProxy{}
	withAutomaticSystemProxy(t)
	s := New()
	s.proxy = p
	defer s.Shutdown()
	if err := s.SetSystemProxy(true); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(direct, freePort(t)); err != nil {
		t.Fatal(err)
	}
	s.handleUnexpectedExit(errors.New("simulated crash"))
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		st := s.Status()
		if !st.Running && !st.RecoveryPending {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("network recovery did not finish after core exit")
}
