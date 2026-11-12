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
	if _, err := os.Stat(m.path); err != nil {
		t.Fatal("journal must remain until all owned groups restore without conflict")
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
