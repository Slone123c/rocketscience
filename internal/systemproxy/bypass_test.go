package systemproxy

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestBypassTakeoverAndRestoration(t *testing.T) {
	cases := []struct {
		name     string
		bypass   *[]string
		expected string
	}{
		{"keep system", nil, `["original"]`},
		{"custom", &[]string{"*.local", "192.168.0.0/16"}, `["*.local","192.168.0.0/16"]`},
		{"explicit empty", &[]string{}, `[]`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := &fakeBackend{services: []Service{{ID: "wifi", Settings: Settings{"__ProtocolEnabled": json.RawMessage("true"), "ExceptionsList": json.RawMessage(`["original"]`)}}}}
			m := New(b, filepath.Join(t.TempDir(), "journal"))
			if err := m.ApplyWithBypass(7890, c.bypass); err != nil {
				t.Fatal(err)
			}
			if got := string(b.services[0].Settings["ExceptionsList"]); got != c.expected {
				t.Fatalf("during takeover %s", got)
			}
			if conflicts, err := m.Restore(); err != nil || len(conflicts) > 0 {
				t.Fatalf("restore: %v %v", conflicts, err)
			}
			if got := string(b.services[0].Settings["ExceptionsList"]); got != `["original"]` {
				t.Fatalf("restored %s", got)
			}
		})
	}
}
func TestBypassExternalChangesPreserved(t *testing.T) {
	b := &fakeBackend{services: []Service{{ID: "wifi", Settings: Settings{"__ProtocolEnabled": json.RawMessage("true"), "ExceptionsList": json.RawMessage(`["old"]`)}}}}
	m := New(b, filepath.Join(t.TempDir(), "journal"))
	bypass := []string{"*.local"}
	if err := m.ApplyWithBypass(7890, &bypass); err != nil {
		t.Fatal(err)
	}
	b.services[0].Settings["ExceptionsList"] = json.RawMessage(`["other-app"]`)
	conflicts, err := m.Restore()
	if err != nil || len(conflicts) == 0 || string(b.services[0].Settings["ExceptionsList"]) != `["other-app"]` {
		t.Fatalf("external edit: %v %v", conflicts, err)
	}
}
