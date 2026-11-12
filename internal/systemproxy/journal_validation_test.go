package systemproxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type journalTrapBackend struct{ called bool }

func (b *journalTrapBackend) Snapshot() ([]Service, error) { b.called = true; return nil, nil }
func (b *journalTrapBackend) CompareAndSwap([]Change) ([]string, error) {
	b.called = true
	return nil, nil
}
func (b *journalTrapBackend) CompareAndSwapPartial([]Change) ([]string, error) {
	b.called = true
	return nil, nil
}

func TestMalformedJournalNeverCallsSystemBackend(t *testing.T) {
	valid := func() Change {
		return Change{ID: "wifi", Keys: []string{"HTTPPort"}, Expected: Settings{"HTTPPort": json.RawMessage("7890")}, Desired: Settings{"HTTPPort": json.RawMessage("8080")}}
	}
	for name, mutate := range map[string]func(*Change){
		"missing ID":         func(c *Change) { c.ID = "" },
		"missing expected":   func(c *Change) { c.Expected = nil },
		"missing desired":    func(c *Change) { c.Desired = nil },
		"empty keys":         func(c *Change) { c.Keys = nil },
		"unknown key":        func(c *Change) { c.Keys = []string{"UnrelatedSetting"} },
		"duplicate key":      func(c *Change) { c.Keys = []string{"HTTPPort", "HTTPPort"} },
		"unknown guard":      func(c *Change) { c.GuardKeys = []string{"UnrelatedSetting"} },
		"unknown setting":    func(c *Change) { c.Desired["UnrelatedSetting"] = json.RawMessage("1") },
		"string port":        func(c *Change) { c.Desired["HTTPPort"] = json.RawMessage(`"8080"`) },
		"fractional port":    func(c *Change) { c.Desired["HTTPPort"] = json.RawMessage("1.5") },
		"large port":         func(c *Change) { c.Desired["HTTPPort"] = json.RawMessage("65536") },
		"invalid protocol":   func(c *Change) { c.Desired["__ProtocolEnabled"] = json.RawMessage("1") },
		"invalid enable":     func(c *Change) { c.Desired["HTTPEnable"] = json.RawMessage("2") },
		"null host":          func(c *Change) { c.Desired["HTTPProxy"] = json.RawMessage("null") },
		"invalid exceptions": func(c *Change) { c.Desired["ExceptionsList"] = json.RawMessage(`["local",1]`) },
	} {
		t.Run(name, func(t *testing.T) {
			change := valid()
			mutate(&change)
			data, err := json.Marshal(journal{Version: 1, Changes: []Change{change}})
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "recovery.json")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			backend := &journalTrapBackend{}
			manager := New(backend, path)
			if pending, err := manager.Pending(); err == nil || pending {
				t.Fatal("invalid journal reported usable")
			}
			if _, err := manager.Restore(); err == nil {
				t.Fatal("invalid journal restored")
			}
			if backend.called {
				t.Fatal("invalid journal reached authorization/system backend")
			}
			if after, err := os.ReadFile(path); err != nil || string(after) != string(data) {
				t.Fatal("invalid recovery evidence was removed or changed")
			}
		})
	}
}

func TestJournalAcceptsAbsentOriginalFieldsAndNativeTypes(t *testing.T) {
	changes := []Change{{ID: "service", Keys: []string{"HTTPEnable", "HTTPProxy", "HTTPPort", "ExceptionsList"}, Expected: Settings{}, Desired: Settings{
		"HTTPEnable": json.RawMessage("true"), "HTTPProxy": json.RawMessage(`"127.0.0.1"`), "HTTPPort": json.RawMessage("7890"), "ExceptionsList": json.RawMessage(`[]`), "__ProtocolEnabled": json.RawMessage("true"),
	}}}
	if err := validateJournalChanges(changes); err != nil {
		t.Fatal(err)
	}
	changes = []Change{{ID: "gnome", Keys: []string{"__Gnome.IgnoreHosts"}, Expected: Settings{"__Gnome.IgnoreHosts": json.RawMessage(`["corp.lan"]`)}, Desired: Settings{"__Gnome.IgnoreHosts": json.RawMessage(`[]`)}}}
	if err := validateJournalChanges(changes); err != nil {
		t.Fatalf("gnome ignore hosts: %v", err)
	}
}
