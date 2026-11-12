package systemproxy

import (
	"encoding/json"
	"testing"
)

func TestWinINETProxyServerRoundTrip(t *testing.T) {
	line := proxyServerLine(7890)
	host, http, https, socks := parseWinINETProxyServer(line)
	if host != "127.0.0.1" || http != 7890 || https != 7890 || socks != 7890 {
		t.Fatalf("parse line: %s", line)
	}
	settings := settingsFromWinINET(winINETSnapshot{ProxyEnable: true, ProxyServer: line, ProxyOverride: "localhost;127.0.0.1"})
	if !parseBool(settings["HTTPEnable"]) || parsePort(settings["HTTPPort"]) != 7890 {
		t.Fatalf("settings: %v", settings)
	}
	next, err := winINETFromSettings(settings)
	if err != nil || next.ProxyServer != line {
		t.Fatalf("round trip: %v %v", next, err)
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
