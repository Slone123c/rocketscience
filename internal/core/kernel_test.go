package core

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestKernelInfoEmbeddedWithoutExternalBinary(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", filepath.Join(t.TempDir(), "missing"))
	s := newTestService(t)
	info, err := s.KernelInfo()
	if err != nil || !info.Available || !strings.HasPrefix(info.Version, "Mihomo ") || info.Platform == "" {
		t.Fatalf("embedded kernel info failed: %+v %v", info, err)
	}
	if s.Status().Running || s.proxy.(*fakeProxy).pending {
		t.Fatal("version query started traffic takeover")
	}
	if err := s.Restart(); err == nil {
		t.Fatal("restart started stopped core")
	}
}

type restartProxy struct {
	fakeProxy
	beforeApply func() error
}

func (p *restartProxy) ApplyWithBypass(port int, bypass *[]string) error {
	if p.beforeApply != nil {
		if err := p.beforeApply(); err != nil {
			return err
		}
	}
	return p.fakeProxy.ApplyWithBypass(port, bypass)
}

func TestKernelRestartPreservesSession(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", filepath.Join(t.TempDir(), "missing"))
	s := newTestService(t)
	p := &restartProxy{}
	s.proxy = p
	defer s.Shutdown()
	content := "mode: rule\nlog-level: silent\nproxy-groups:\n  - name: choose / 节点\n    type: select\n    proxies: [DIRECT, REJECT]\nrules: ['MATCH,DIRECT']\n"
	port := freePort(t)
	if err := s.Start(content, port); err != nil {
		t.Fatal(err)
	}
	if err := s.Select("choose / 节点", "REJECT"); err != nil {
		t.Fatal(err)
	}
	if err := s.Select("GLOBAL", "choose / 节点"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMode("global"); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	oldStart, revision := s.startedAt, s.modeRevision
	s.mu.Unlock()
	p.beforeApply = func() error {
		if currentInProcessMode() != "global" {
			return errors.New("takeover before restoring mode")
		}
		proxies, err := inProcessProxiesMap()
		if err != nil {
			return err
		}
		if proxies["choose / 节点"].Now != "REJECT" || proxies["GLOBAL"].Now != "choose / 节点" {
			return errors.New("takeover before restoring selections")
		}
		return nil
	}
	if err := s.Restart(); err != nil {
		t.Fatal(err)
	}
	st := s.Status()
	if !st.Running || st.Mode != "global" || st.MixedPort != port || !st.SystemProxy || !st.StartedAt.After(oldStart) {
		t.Fatalf("restart state: %+v", st)
	}
	s.mu.Lock()
	if s.modeRevision != revision {
		t.Error("internal restart invalidated Wi-Fi ownership")
	}
	s.mu.Unlock()
	if p.restored != 1 || !p.pending {
		t.Fatal("restart did not restore then reacquire proxy")
	}
	p.restoreErr = errors.New("restoration denied")
	if err := s.Restart(); err == nil {
		t.Fatal("restart ignored restoration failure")
	}
	if next := s.Status(); !next.Running || next.StartedAt != st.StartedAt {
		t.Fatal("restoration failure terminated core")
	}
	p.restoreErr = nil
}

func TestKernelRestartFailureLeavesNetworkRestored(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", filepath.Join(t.TempDir(), "missing"))
	s := newTestService(t)
	defer s.Shutdown()
	if err := s.Start(direct, freePort(t)); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.currentContent = "not: [yaml"
	s.mu.Unlock()
	if err := s.Restart(); err == nil {
		t.Fatal("restart unexpectedly succeeded")
	}
	if !s.Status().Running || !s.proxy.(*fakeProxy).pending {
		t.Fatal("invalid config stopped the running core")
	}
	s.mu.Lock()
	s.currentContent = direct
	s.mu.Unlock()
	if err := s.Restart(); err != nil {
		t.Fatalf("valid config could not restart: %v", err)
	}
}

func TestKernelSelectionFailureDoesNotTakeOver(t *testing.T) {
	t.Setenv("ROCKETSCIENCE_MIHOMO", filepath.Join(t.TempDir(), "missing"))
	s := newTestService(t)
	defer s.Shutdown()
	s.opMu.Lock()
	err := s.startLocked(direct, freePort(t), nil, LANSettings{}, CoreOptions{}, map[string]string{"missing group": "missing node"})
	s.opMu.Unlock()
	if err == nil {
		t.Fatal("invalid selection accepted")
	}
	p := s.proxy.(*fakeProxy)
	if s.Status().Running || p.pending || p.applied != 0 {
		t.Fatal("selection failure took over traffic or left core running")
	}
}
