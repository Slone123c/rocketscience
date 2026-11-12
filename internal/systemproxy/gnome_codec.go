//go:build linux

package systemproxy

const (
	gnomeKeyAutoURL     = "__Gnome.AutoConfigURL"
	gnomeKeyIgnoreHosts = "__Gnome.IgnoreHosts"
	gnomeKeyMode        = "__Gnome.Mode"
)

func settingsFromGnome(s gnomeSnapshot) Settings {
	enabled := s.Mode == "manual" && (s.HTTPPort > 0 || s.HTTPSPort > 0 || s.SOCKSPort > 0)
	settings := Settings{
		"__ProtocolEnabled":        rawJSON(enabled),
		gnomeKeyMode:               rawString(s.Mode),
		gnomeKeyAutoURL:            rawString(s.AutoURL),
		gnomeKeyIgnoreHosts:        rawJSON(s.IgnoreHosts),
		"HTTPEnable":               rawBool(enabled && s.HTTPPort > 0),
		"HTTPProxy":                rawString(s.HTTPHost),
		"HTTPPort":                 rawJSON(s.HTTPPort),
		"HTTPSEnable":              rawBool(enabled && s.HTTPSPort > 0),
		"HTTPSProxy":               rawString(s.HTTPSHost),
		"HTTPSPort":                rawJSON(s.HTTPSPort),
		"SOCKSEnable":              rawBool(enabled && s.SOCKSPort > 0),
		"SOCKSProxy":               rawString(s.SOCKSHost),
		"SOCKSPort":                rawJSON(s.SOCKSPort),
		"ProxyAutoConfigEnable":    rawBool(s.Mode == "auto" || s.AutoURL != ""),
		"ProxyAutoDiscoveryEnable": rawBool(false),
	}
	if len(s.IgnoreHosts) > 0 {
		settings["ExceptionsList"] = rawJSON(s.IgnoreHosts)
	}
	return settings
}

func applyGnomeChanges(current gnomeSnapshot, merged Settings, changes []Change) gnomeSnapshot {
	next := current
	touched := map[string]bool{}
	for _, c := range changes {
		for _, k := range c.Keys {
			touched[k] = true
		}
	}
	if touched["__ProtocolEnabled"] || touched["HTTPEnable"] || touched["HTTPSEnable"] || touched["SOCKSEnable"] {
		if parseBool(merged["__ProtocolEnabled"]) ||
			parseBool(merged["HTTPEnable"]) || parseBool(merged["HTTPSEnable"]) || parseBool(merged["SOCKSEnable"]) {
			next.Mode = "manual"
			next.HTTPHost = parseString(merged["HTTPProxy"])
			next.HTTPPort = parsePort(merged["HTTPPort"])
			next.HTTPSHost = parseString(merged["HTTPSProxy"])
			next.HTTPSPort = parsePort(merged["HTTPSPort"])
			next.SOCKSHost = parseString(merged["SOCKSProxy"])
			next.SOCKSPort = parsePort(merged["SOCKSPort"])
		} else if touched["__ProtocolEnabled"] {
			next.Mode = "none"
		}
	}
	if touched["ProxyAutoConfigEnable"] {
		if parseBool(merged["ProxyAutoConfigEnable"]) {
			next.Mode = "auto"
			next.AutoURL = parseString(merged[gnomeKeyAutoURL])
		} else {
			next.AutoURL = ""
			if next.Mode == "auto" {
				next.Mode = "none"
			}
		}
	}
	if touched["ExceptionsList"] {
		next.IgnoreHosts = parseExceptions(merged["ExceptionsList"])
	}
	return next
}
