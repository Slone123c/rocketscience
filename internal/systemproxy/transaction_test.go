package systemproxy

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type fakeBackend struct {
	services []Service
	fail     bool
}

func (b *fakeBackend) Snapshot() ([]Service, error) { return b.services, nil }
func (b *fakeBackend) CompareAndSwap(changes []Change) ([]string, error) {
	return b.cas(changes, false)
}

func (b *fakeBackend) CompareAndSwapPartial(changes []Change) ([]string, error) {
	return b.cas(changes, true)
}

func (b *fakeBackend) cas(changes []Change, partial bool) ([]string, error) {
	if b.fail {
		return nil, errors.New("write failed")
	}
	conflicts, updated, applied := applyChanges(b.services, changes)
	if len(conflicts) > 0 && !partial {
		return conflicts, nil
	}
	if len(applied) == 0 {
		return conflicts, nil
	}
	b.services = updated
	return conflicts, nil
}
func TestRoundTripPreservesOriginalAndUnrelatedFields(t *testing.T) {
	b := &fakeBackend{services: []Service{{ID: "wifi", Settings: Settings{"__ProtocolEnabled": json.RawMessage("false"), "HTTPEnable": json.RawMessage("1"), "HTTPProxy": json.RawMessage(`"old"`), "HTTPPort": json.RawMessage("8080"), "ExceptionsList": json.RawMessage(`["localhost"]`)}}}}
	path := filepath.Join(t.TempDir(), "journal.json")
	m := New(b, path)
	guardKeys := append(append([]string{}, keys[:len(keys)-2]...), "__ProtocolEnabled")
	before := pickKeys(b.services[0].Settings, guardKeys)
	if err := m.Apply(7890); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil || fi.Mode().Perm() != 0600 {
		t.Fatalf("journal permissions: %v %v", fi, err)
	}
	b.services[0].Settings["ExceptionsList"] = json.RawMessage(`["later"]`)
	conflicts, err := New(b, path).Restore()
	if err != nil || len(conflicts) > 0 {
		t.Fatalf("restore: %v %v", conflicts, err)
	}
	if !reflect.DeepEqual(before, pickKeys(b.services[0].Settings, guardKeys)) {
		t.Fatalf("original not restored: %v", b.services[0].Settings)
	}
	if string(b.services[0].Settings["ExceptionsList"]) != `["later"]` {
		t.Fatal("unrelated settings overwritten")
	}
	if _, err = os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("journal not removed")
	}
}
func TestExternalChangeIsNotOverwritten(t *testing.T) {
	b := &fakeBackend{services: []Service{{ID: "wifi", Settings: Settings{"__ProtocolEnabled": json.RawMessage("false")}}}}
	m := New(b, filepath.Join(t.TempDir(), "journal"))
	if err := m.Apply(7890); err != nil {
		t.Fatal(err)
	}
	b.services[0].Settings["HTTPPort"] = json.RawMessage("9999")
	conflicts, err := m.Restore()
	if err != nil || len(conflicts) == 0 || string(b.services[0].Settings["HTTPPort"]) != "9999" {
		t.Fatalf("external change lost: %v %v", conflicts, err)
	}
	if _, ok := b.services[0].Settings["HTTPSProxy"]; ok {
		t.Fatal("partial restore should revert unconflicted owned proxy groups")
	}
	if _, err := os.Stat(m.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("journal removed after successful partial restore (conflicts returned)")
	}
	if err := New(b, m.path).Apply(7890); err != nil {
		t.Fatal("fresh takeover after partial restore with conflicts")
	}
}
func TestFailedApplyKeepsRecoveryRecord(t *testing.T) {
	b := &fakeBackend{services: []Service{{ID: "wifi", Settings: Settings{"__ProtocolEnabled": json.RawMessage("false")}}}, fail: true}
	path := filepath.Join(t.TempDir(), "journal")
	m := New(b, path)
	if err := m.Apply(7890); err == nil {
		t.Fatal("failure not returned")
	}
	if ok, err := m.Pending(); err != nil || !ok {
		t.Fatalf("missing recovery record: %v %v", ok, err)
	}
	b.fail = false
	if _, err := m.Restore(); err != nil {
		t.Fatal(err)
	}
}

func TestRecoveryRecordIsExclusive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal")
	first := New(&fakeBackend{}, path)
	second := New(&fakeBackend{}, path)
	j := journal{Version: 1, Changes: []Change{{ID: "wifi"}}}
	results := make(chan error, 2)
	go func() { results <- first.write(j) }()
	go func() { results <- second.write(j) }()
	a, b := <-results, <-results
	if !((a == nil && errors.Is(b, ErrAlreadyPending)) || (b == nil && errors.Is(a, ErrAlreadyPending))) {
		t.Fatalf("expected one owner: %v, %v", a, b)
	}
}

func TestSkipsServiceWithoutProxyProtocol(t *testing.T) {
	b := &fakeBackend{services: []Service{
		{ID: "vpn-without-proxies", Settings: Settings{}},
		{ID: "wifi", Settings: Settings{"__ProtocolEnabled": json.RawMessage("false")}},
	}}
	m := New(b, filepath.Join(t.TempDir(), "journal"))
	if err := m.Apply(7890); err != nil {
		t.Fatalf("Apply failed when one service lacked proxy protocol: %v", err)
	}
	if string(b.services[1].Settings["HTTPPort"]) != "7890" {
		t.Fatalf("wifi proxy not applied: %v", b.services[1].Settings)
	}
	if _, err := m.Restore(); err != nil {
		t.Fatal(err)
	}
}

func TestEmptyCustomBypassRetainsLoopbackAndLocalExceptions(t *testing.T) {
	b := &fakeBackend{services: []Service{
		{ID: "wifi", Settings: Settings{"__ProtocolEnabled": json.RawMessage("false"), "ExceptionsList": json.RawMessage(`["corp.internal"]`)}},
	}}
	m := New(b, filepath.Join(t.TempDir(), "journal"))
	empty := []string{}
	if err := m.ApplyWithBypass(7890, &empty); err != nil {
		t.Fatal(err)
	}
	var got []string
	if err := json.Unmarshal(b.services[0].Settings["ExceptionsList"], &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"127.0.0.1", "localhost", "*.local", "169.254/16"}) {
		t.Fatalf("expected default loopback/local exceptions to be retained, got: %v", got)
	}
	if _, err := m.Restore(); err != nil {
		t.Fatal(err)
	}
	if string(b.services[0].Settings["ExceptionsList"]) != `["corp.internal"]` {
		t.Fatalf("original ExceptionsList not restored: %s", b.services[0].Settings["ExceptionsList"])
	}
}

func TestRestorePartialConflictReleasesOwnershipAndDeletesJournal(t *testing.T) {
	b := &fakeBackend{services: []Service{{ID: "wifi", Settings: Settings{
		"__ProtocolEnabled":        json.RawMessage("false"),
		"HTTPEnable":               json.RawMessage("1"),
		"HTTPProxy":                json.RawMessage(`"corp-http"`),
		"HTTPPort":                 json.RawMessage("8080"),
		"HTTPSEnable":              json.RawMessage("1"),
		"HTTPSProxy":               json.RawMessage(`"corp-https"`),
		"HTTPSPort":                json.RawMessage("8443"),
		"SOCKSEnable":              json.RawMessage("0"),
		"ProxyAutoConfigEnable":    json.RawMessage("0"),
		"ProxyAutoDiscoveryEnable": json.RawMessage("0"),
	}}}}
	journalPath := filepath.Join(t.TempDir(), "journal.json")
	m := New(b, journalPath)
	if err := m.Apply(7890); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(journalPath); err != nil {
		t.Fatalf("expected journal after apply: %v", err)
	}

	// User externally edits HTTPSProxy; HTTP/SOCKS remain owned by RocketScience.
	b.services[0].Settings["HTTPSProxy"] = json.RawMessage(`"user-https"`)
	b.services[0].Settings["HTTPSPort"] = json.RawMessage("9443")

	conflicts, err := m.Restore()
	if err != nil {
		t.Fatalf("partial restore returned error: %v", err)
	}
	if len(conflicts) == 0 {
		t.Fatal("expected conflict for externally edited HTTPS group")
	}
	// (a) Owned HTTP group must be restored and journal removed.
	if string(b.services[0].Settings["HTTPProxy"]) != `"corp-http"` || string(b.services[0].Settings["HTTPPort"]) != "8080" {
		t.Fatalf("owned HTTP group not restored: %+v", b.services[0].Settings)
	}
	if _, err := os.Stat(journalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("journal must be removed after partial restore, stat err=%v", err)
	}
	if pending, err := m.Pending(); err != nil || pending {
		t.Fatalf("Pending=%v err=%v, want false", pending, err)
	}
	// (b) Externally edited HTTPS group must NOT be overwritten, and ownership is released (second Restore is a no-op).
	if string(b.services[0].Settings["HTTPSProxy"]) != `"user-https"` || string(b.services[0].Settings["HTTPSPort"]) != "9443" {
		t.Fatalf("external edit was overwritten: %+v", b.services[0].Settings)
	}
	if conflicts2, err := New(b, journalPath).Restore(); err != nil || len(conflicts2) != 0 {
		t.Fatalf("second Restore should be no-op, got conflicts=%v err=%v", conflicts2, err)
	}
	if string(b.services[0].Settings["HTTPSProxy"]) != `"user-https"` {
		t.Fatalf("second Restore mutated external edit: %+v", b.services[0].Settings)
	}
	// (d) Apply -> Restore -> Apply again must work.
	if err := m.Apply(7891); err != nil {
		t.Fatalf("re-apply after partial restore failed: %v", err)
	}
	if string(b.services[0].Settings["HTTPPort"]) != "7891" || string(b.services[0].Settings["HTTPSPort"]) != "7891" {
		t.Fatalf("re-apply did not take over: %+v", b.services[0].Settings)
	}
}

func TestRestoreFailurePreservesJournalForFreshManagerRecovery(t *testing.T) {
	b := &fakeBackend{services: []Service{{ID: "wifi", Settings: Settings{
		"__ProtocolEnabled":        json.RawMessage("false"),
		"HTTPEnable":               json.RawMessage("1"),
		"HTTPProxy":                json.RawMessage(`"corp-http"`),
		"HTTPPort":                 json.RawMessage("8080"),
		"ProxyAutoConfigEnable":    json.RawMessage("0"),
		"ProxyAutoDiscoveryEnable": json.RawMessage("0"),
	}}}}
	journalPath := filepath.Join(t.TempDir(), "journal.json")
	m1 := New(b, journalPath)
	if err := m1.Apply(7890); err != nil {
		t.Fatal(err)
	}

	b.fail = true
	if _, err := m1.Restore(); err == nil {
		t.Fatal("expected error when backend write fails")
	}
	// Journal must still exist and be readable when backend fails.
	raw, err := os.ReadFile(journalPath)
	if err != nil || len(raw) == 0 {
		t.Fatalf("journal must remain intact on restore failure: len=%d err=%v", len(raw), err)
	}
	if pending, err := m1.Pending(); err != nil || !pending {
		t.Fatalf("Pending=%v err=%v, want true", pending, err)
	}

	// (c) A fresh Manager started later recovers still-owned leftovers from the journal.
	b.fail = false
	m2 := New(b, journalPath)
	if pending, err := m2.Pending(); err != nil || !pending {
		t.Fatalf("fresh Manager Pending=%v err=%v, want true", pending, err)
	}
	conflicts, err := m2.Restore()
	if err != nil || len(conflicts) > 0 {
		t.Fatalf("fresh Manager Restore failed: conflicts=%v err=%v", conflicts, err)
	}
	if string(b.services[0].Settings["HTTPProxy"]) != `"corp-http"` || string(b.services[0].Settings["HTTPPort"]) != "8080" {
		t.Fatalf("fresh Manager did not restore original settings: %+v", b.services[0].Settings)
	}
	if _, err := os.Stat(journalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("journal must be removed after fresh Manager recovery, stat err=%v", err)
	}
	// (d) Apply again works after recovery.
	if err := m2.Apply(7892); err != nil {
		t.Fatalf("re-apply after recovery failed: %v", err)
	}
}
