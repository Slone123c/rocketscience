package systemproxy

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const gnomeServiceID = "gnome"

type gnomeSnapshot struct {
	Mode        string
	HTTPHost    string
	HTTPPort    int
	HTTPSHost   string
	HTTPSPort   int
	SOCKSHost   string
	SOCKSPort   int
	IgnoreHosts []string
	AutoURL     string
}

func settingsFromGnome(s gnomeSnapshot) Settings {
	enabled := s.Mode == "manual" && (s.HTTPPort > 0 || s.HTTPSPort > 0 || s.SOCKSPort > 0)
	settings := Settings{"__ProtocolEnabled": rawJSON(enabled)}
	if !enabled {
		settings["HTTPEnable"] = rawBool(false)
		settings["HTTPSEnable"] = rawBool(false)
		settings["SOCKSEnable"] = rawBool(false)
		settings["ProxyAutoConfigEnable"] = rawBool(s.Mode == "auto" || s.AutoURL != "")
		settings["ProxyAutoDiscoveryEnable"] = rawBool(false)
		if len(s.IgnoreHosts) > 0 {
			settings["ExceptionsList"] = rawJSON(s.IgnoreHosts)
		}
		return settings
	}
	host := s.HTTPHost
	if host == "" {
		host = s.HTTPSHost
	}
	if host == "" {
		host = s.SOCKSHost
	}
	settings["HTTPEnable"] = rawBool(s.HTTPPort > 0)
	settings["HTTPProxy"] = rawString(host)
	settings["HTTPPort"] = rawJSON(s.HTTPPort)
	settings["HTTPSEnable"] = rawBool(s.HTTPSPort > 0)
	settings["HTTPSProxy"] = rawString(s.HTTPSHost)
	settings["HTTPSPort"] = rawJSON(s.HTTPSPort)
	settings["SOCKSEnable"] = rawBool(s.SOCKSPort > 0)
	settings["SOCKSProxy"] = rawString(s.SOCKSHost)
	settings["SOCKSPort"] = rawJSON(s.SOCKSPort)
	settings["ProxyAutoConfigEnable"] = rawBool(s.Mode == "auto" || s.AutoURL != "")
	settings["ProxyAutoDiscoveryEnable"] = rawBool(false)
	if len(s.IgnoreHosts) > 0 {
		settings["ExceptionsList"] = rawJSON(s.IgnoreHosts)
	}
	return settings
}

func gnomeFromSettings(settings Settings) (gnomeSnapshot, error) {
	enabled := parseBool(settings["__ProtocolEnabled"])
	if v, ok := settings["HTTPEnable"]; ok && parseBool(v) {
		enabled = true
	}
	s := gnomeSnapshot{Mode: "none"}
	if enabled {
		s.Mode = "manual"
		s.HTTPHost = parseString(settings["HTTPProxy"])
		s.HTTPPort = parsePort(settings["HTTPPort"])
		s.HTTPSHost = parseString(settings["HTTPSProxy"])
		s.HTTPSPort = parsePort(settings["HTTPSPort"])
		s.SOCKSHost = parseString(settings["SOCKSProxy"])
		s.SOCKSPort = parsePort(settings["SOCKSPort"])
		if s.HTTPPort == 0 && s.HTTPSPort == 0 && s.SOCKSPort == 0 {
			return s, fmt.Errorf("代理端口无效")
		}
		if s.HTTPHost == "" {
			s.HTTPHost = "127.0.0.1"
		}
		if s.HTTPSHost == "" {
			s.HTTPSHost = s.HTTPHost
		}
		if s.SOCKSHost == "" {
			s.SOCKSHost = s.HTTPHost
		}
	} else if parseBool(settings["ProxyAutoConfigEnable"]) {
		s.Mode = "auto"
	}
	if raw, ok := settings["ExceptionsList"]; ok {
		s.IgnoreHosts = parseExceptions(raw)
	}
	return s, nil
}

func parseGsettingsValue(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if raw == "true" {
		return "true", nil
	}
	if raw == "false" {
		return "false", nil
	}
	if (strings.HasPrefix(raw, "'") && strings.HasSuffix(raw, "'")) || (strings.HasPrefix(raw, `"`) && strings.HasSuffix(raw, `"`)) {
		var s string
		if err := json.Unmarshal([]byte(strings.ReplaceAll(raw, "'", `"`)), &s); err == nil {
			return s, nil
		}
		return strings.Trim(raw, `'"`), nil
	}
	return raw, nil
}

func parseGsettingsStringArray(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "@as []" {
		return nil, nil
	}
	// gsettings prints ['a', 'b'] — normalize to JSON.
	if strings.HasPrefix(raw, "[") || strings.HasPrefix(raw, "@as") {
		normalized := strings.ReplaceAll(raw, "'", `"`)
		var items []string
		if err := json.Unmarshal([]byte(normalized), &items); err != nil {
			return nil, fmt.Errorf("无法解析忽略列表：%s", raw)
		}
		return items, nil
	}
	return nil, nil
}

func formatGsettingsString(value string) string {
	b, _ := json.Marshal(value)
	return string(b)
}

func formatGsettingsStringArray(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = formatGsettingsString(item)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func formatGsettingsUInt(value int) string {
	return strconv.Itoa(value)
}
