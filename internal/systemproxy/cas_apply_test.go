package systemproxy

import (
	"encoding/json"
	"testing"
)

func TestApplyChangesValidatesAgainstPreTransactionSnapshot(t *testing.T) {
	original := Settings{
		"__ProtocolEnabled":        json.RawMessage("false"),
		"HTTPEnable":               json.RawMessage("0"),
		"HTTPProxy":                json.RawMessage(`"corp.example"`),
		"HTTPPort":                 json.RawMessage("8080"),
		"HTTPSEnable":              json.RawMessage("0"),
		"HTTPSProxy":               json.RawMessage(`"secure.corp"`),
		"HTTPSPort":                json.RawMessage("8443"),
		"SOCKSEnable":              json.RawMessage("0"),
		"SOCKSProxy":               json.RawMessage(`"socks.corp"`),
		"SOCKSPort":                json.RawMessage("1080"),
		"ProxyAutoConfigEnable":    json.RawMessage("1"),
		"ProxyAutoDiscoveryEnable": json.RawMessage("1"),
		"ExceptionsList":           json.RawMessage(`["intranet.corp"]`),
	}
	const serviceID = "wininet"
	services := []Service{{ID: serviceID, Settings: cloneSettingsMap(original)}}
	desired := takeover(7890)
	var changes []Change
	for _, group := range keyGroups {
		changes = append(changes, Change{
			ID:       serviceID,
			Expected: pickKeys(original, group),
			Desired:  pickKeys(desired, group),
			Keys:     append([]string(nil), group...),
		})
	}
	guardKeys := append(append([]string{}, keys[:len(keys)-2]...), "__ProtocolEnabled")
	changes = append(changes, Change{
		ID:        serviceID,
		Expected:  pickKeys(original, guardKeys),
		Desired:   desired,
		Keys:      []string{"__ProtocolEnabled"},
		GuardKeys: guardKeys,
	})
	conflicts, updated, _ := applyChanges(services, changes)
	if len(conflicts) > 0 {
		t.Fatalf("journal-shaped batch must not conflict after prior in-batch mutations: %v", conflicts)
	}
	if !parseBool(updated[0].Settings["__ProtocolEnabled"]) {
		t.Fatal("__ProtocolEnabled not applied")
	}
	if parseString(updated[0].Settings["HTTPProxy"]) != "127.0.0.1" {
		t.Fatalf("HTTPProxy=%q", updated[0].Settings["HTTPProxy"])
	}
}

func TestApplyChangesDetectsExternalMutation(t *testing.T) {
	baseline := Settings{"__ProtocolEnabled": json.RawMessage("false"), "HTTPPort": json.RawMessage("8080")}
	services := []Service{{ID: "wifi", Settings: baseline}}
	changes := []Change{{
		ID:       "wifi",
		Expected: Settings{"HTTPPort": json.RawMessage("8080")},
		Desired:  Settings{"HTTPPort": json.RawMessage("7890")},
		Keys:     []string{"HTTPPort"},
	}}
	// Simulate concurrent edit before CAS: snapshot at write time differs from journal Expected.
	services[0].Settings["HTTPPort"] = json.RawMessage("9999")
	conflicts, _, _ := applyChanges(services, changes)
	if len(conflicts) == 0 {
		t.Fatal("expected conflict when live snapshot diverged from journal expected")
	}
}
