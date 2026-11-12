//go:build windows

package systemproxy

const (
	winINETKeyProxyServerRaw = "__WinINET.ProxyServer"
	winINETKeyAutoConfigURL  = "__WinINET.AutoConfigURL"
)

func settingsFromWinINET(s winINETSnapshot) Settings {
	ep := parseWinINETEndpointsPortable(s.ProxyServer)
	active := s.ProxyEnable
	settings := Settings{
		"__ProtocolEnabled":        rawJSON(active),
		winINETKeyProxyServerRaw:   rawString(s.ProxyServer),
		winINETKeyAutoConfigURL:    rawString(s.AutoConfigURL),
		"HTTPEnable":               rawBool(active && ep.HTTPPort > 0),
		"HTTPProxy":                rawString(ep.HTTPHost),
		"HTTPPort":                 rawJSON(ep.HTTPPort),
		"HTTPSEnable":              rawBool(active && ep.HTTPSPort > 0),
		"HTTPSProxy":               rawString(ep.HTTPSHost),
		"HTTPSPort":                rawJSON(ep.HTTPSPort),
		"SOCKSEnable":              rawBool(active && ep.SOCKSPort > 0),
		"SOCKSProxy":               rawString(ep.SOCKSHost),
		"SOCKSPort":                rawJSON(ep.SOCKSPort),
		"ProxyAutoConfigEnable":    rawBool(s.AutoConfigURL != ""),
		"ProxyAutoDiscoveryEnable": rawBool(s.AutoDiscovery),
	}
	if s.ProxyOverride != "" {
		settings["ExceptionsList"] = rawJSON(overrideToExceptions(s.ProxyOverride))
	}
	return settings
}

func applyWinINETChanges(current winINETSnapshot, merged Settings, changes []Change) winINETSnapshot {
	next := current
	touched := map[string]bool{}
	for _, c := range changes {
		for _, k := range c.Keys {
			touched[k] = true
		}
	}
	if touched["__ProtocolEnabled"] || touched["HTTPEnable"] || touched["HTTPSEnable"] || touched["SOCKSEnable"] {
		next.ProxyEnable = parseBool(merged["__ProtocolEnabled"]) ||
			parseBool(merged["HTTPEnable"]) || parseBool(merged["HTTPSEnable"]) || parseBool(merged["SOCKSEnable"])
	}
	if protocolKeysTouched(touched) || touched[winINETKeyProxyServerRaw] {
		if line := buildProxyServerFromSettings(merged); line != "" {
			next.ProxyServer = line
		} else if raw := parseString(merged[winINETKeyProxyServerRaw]); raw != "" {
			next.ProxyServer = raw
		} else {
			next.ProxyServer = ""
		}
	}
	if touched["ExceptionsList"] {
		if raw, ok := merged["ExceptionsList"]; ok {
			next.ProxyOverride = exceptionsToOverride(parseExceptions(raw))
		} else {
			next.ProxyOverride = ""
		}
	}
	if touched["ProxyAutoConfigEnable"] || touched[winINETKeyAutoConfigURL] {
		if parseBool(merged["ProxyAutoConfigEnable"]) {
			next.AutoConfigURL = parseString(merged[winINETKeyAutoConfigURL])
		} else {
			next.AutoConfigURL = ""
		}
	}
	if touched["ProxyAutoDiscoveryEnable"] {
		next.AutoDiscovery = parseBool(merged["ProxyAutoDiscoveryEnable"])
	}
	return next
}

func protocolKeysTouched(touched map[string]bool) bool {
	for _, k := range []string{"HTTPEnable", "HTTPProxy", "HTTPPort", "HTTPSEnable", "HTTPSProxy", "HTTPSPort", "SOCKSEnable", "SOCKSProxy", "SOCKSPort"} {
		if touched[k] {
			return true
		}
	}
	return false
}
