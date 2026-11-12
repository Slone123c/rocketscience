package systemproxy

import (
	"encoding/json"
	"testing"
)

func TestWinINETProxyServerRoundTrip(t *testing.T) {
	line := "http=corp:8080;https=secure:8443;socks=socks:1080"
	ep := parseWinINETEndpointsPortable(line)
	if ep.HTTPHost != "corp" || ep.HTTPPort != 8080 || ep.HTTPSHost != "secure" || ep.HTTPSPort != 8443 {
		t.Fatalf("parse endpoints: %+v", ep)
	}
	built := buildProxyServerFromSettings(Settings{
		"HTTPEnable":  json.RawMessage("1"),
		"HTTPProxy":   json.RawMessage(`"corp"`),
		"HTTPPort":    json.RawMessage("8080"),
		"HTTPSEnable": json.RawMessage("1"),
		"HTTPSProxy":  json.RawMessage(`"secure"`),
		"HTTPSPort":   json.RawMessage("8443"),
		"SOCKSEnable": json.RawMessage("1"),
		"SOCKSProxy":  json.RawMessage(`"socks"`),
		"SOCKSPort":   json.RawMessage("1080"),
	})
	if built != line {
		t.Fatalf("build: %q", built)
	}
}

func TestApplyChangesMatchesTransactionSemantics(t *testing.T) {
	services := []Service{{ID: "wifi", Settings: Settings{
		"__ProtocolEnabled": json.RawMessage("false"),
		"HTTPEnable":        json.RawMessage("1"),
		"HTTPProxy":         json.RawMessage(`"old"`),
		"HTTPPort":          json.RawMessage("8080"),
	}}}
	conflicts, updated := applyChanges(services, []Change{{
		ID: "wifi",
		Expected: Settings{
			"HTTPEnable": json.RawMessage("1"),
			"HTTPProxy":  json.RawMessage(`"old"`),
			"HTTPPort":   json.RawMessage("8080"),
		},
		Desired: Settings{
			"HTTPProxy": json.RawMessage(`"127.0.0.1"`),
			"HTTPPort":  json.RawMessage("7890"),
		},
		Keys: []string{"HTTPProxy", "HTTPPort"},
	}})
	if len(conflicts) > 0 || parseString(updated[0].Settings["HTTPProxy"]) != "127.0.0.1" {
		t.Fatalf("conflicts=%v settings=%v", conflicts, updated[0].Settings)
	}
}
