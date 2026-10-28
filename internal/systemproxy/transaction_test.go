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
	if b.fail {
		return nil, errors.New("write failed")
	}
	var conflicts []string
	baseline := map[string]Settings{}
	for _, service := range b.services {
		clone := Settings{}
		for k, v := range service.Settings {
			clone[k] = append(json.RawMessage(nil), v...)
		}
		baseline[service.ID] = clone
	}
	for _, c := range changes {
		var s *Service
		for i := range b.services {
			if b.services[i].ID == c.ID {
				s = &b.services[i]
				break
			}
		}
		if s == nil {
			conflicts = append(conflicts, c.ID)
			continue
		}
		matches := true
		checkKeys := c.Keys
		if len(c.GuardKeys) > 0 {
			checkKeys = c.GuardKeys
		}
		for _, k := range checkKeys {
			a, aok := baseline[c.ID][k]
			v, vok := c.Expected[k]
			if aok != vok || string(a) != string(v) {
				matches = false
			}
		}
		if !matches {
			conflicts = append(conflicts, c.ID)
			continue
		}
		for _, k := range c.Keys {
			if v, ok := c.Desired[k]; ok {
				s.Settings[k] = append(json.RawMessage(nil), v...)
			} else {
				delete(s.Settings, k)
			}
		}
	}
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
		t.Fatal("unrelated owned proxy was not restored")
	}
	if string(b.services[0].Settings["__ProtocolEnabled"]) != "true" {
		t.Fatal("external proxy change was disabled")
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
