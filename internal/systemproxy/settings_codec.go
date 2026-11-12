package systemproxy

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func rawBool(enabled bool) json.RawMessage {
	if enabled {
		return json.RawMessage("1")
	}
	return json.RawMessage("0")
}

func rawString(value string) json.RawMessage {
	b, _ := json.Marshal(value)
	return b
}

func rawJSON(value any) json.RawMessage {
	b, _ := json.Marshal(value)
	return b
}

func parseBool(raw json.RawMessage) bool {
	var n int
	if json.Unmarshal(raw, &n) == nil {
		return n != 0
	}
	var b bool
	_ = json.Unmarshal(raw, &b)
	return b
}

func parseString(raw json.RawMessage) string {
	var s string
	_ = json.Unmarshal(raw, &s)
	return s
}

func parsePort(raw json.RawMessage) int {
	var n int
	if json.Unmarshal(raw, &n) != nil {
		return 0
	}
	return n
}

func parseExceptions(raw json.RawMessage) []string {
	var items []string
	if len(raw) == 0 {
		return nil
	}
	_ = json.Unmarshal(raw, &items)
	return items
}

// proxyServerLine builds the WinINET ProxyServer value RocketScience applies.
func proxyServerLine(port int) string {
	host := "127.0.0.1"
	p := strconv.Itoa(port)
	return fmt.Sprintf("http=%s:%s;https=%s:%s;socks=%s:%s", host, p, host, p, host, p)
}

func exceptionsToOverride(entries []string) string {
	if len(entries) == 0 {
		return ""
	}
	return strings.Join(entries, ";")
}

func overrideToExceptions(override string) []string {
	override = strings.TrimSpace(override)
	if override == "" {
		return nil
	}
	parts := strings.Split(override, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

type winINETSnapshot struct {
	ProxyEnable    bool
	ProxyServer    string
	ProxyOverride  string
	AutoConfigURL  string
	AutoDiscovery  bool
}

func settingsFromWinINET(s winINETSnapshot) Settings {
	settings := Settings{
		"__ProtocolEnabled": rawJSON(s.ProxyEnable),
	}
	if !s.ProxyEnable {
		settings["HTTPEnable"] = rawBool(false)
		settings["HTTPSEnable"] = rawBool(false)
		settings["SOCKSEnable"] = rawBool(false)
		settings["ProxyAutoConfigEnable"] = rawBool(s.AutoConfigURL != "")
		settings["ProxyAutoDiscoveryEnable"] = rawBool(s.AutoDiscovery)
		if len(s.ProxyOverride) > 0 {
			settings["ExceptionsList"] = rawJSON(overrideToExceptions(s.ProxyOverride))
		}
		return settings
	}
	host, httpPort, httpsPort, socksPort := parseWinINETProxyServer(s.ProxyServer)
	settings["HTTPEnable"] = rawBool(httpPort > 0)
	settings["HTTPProxy"] = rawString(host)
	settings["HTTPPort"] = rawJSON(httpPort)
	settings["HTTPSEnable"] = rawBool(httpsPort > 0)
	settings["HTTPSProxy"] = rawString(host)
	settings["HTTPSPort"] = rawJSON(httpsPort)
	settings["SOCKSEnable"] = rawBool(socksPort > 0)
	settings["SOCKSProxy"] = rawString(host)
	settings["SOCKSPort"] = rawJSON(socksPort)
	settings["ProxyAutoConfigEnable"] = rawBool(s.AutoConfigURL != "")
	settings["ProxyAutoDiscoveryEnable"] = rawBool(s.AutoDiscovery)
	if len(s.ProxyOverride) > 0 {
		settings["ExceptionsList"] = rawJSON(overrideToExceptions(s.ProxyOverride))
	}
	return settings
}

func parseWinINETProxyServer(line string) (host string, httpPort, httpsPort, socksPort int) {
	host = "127.0.0.1"
	line = strings.TrimSpace(line)
	if line == "" {
		return host, 0, 0, 0
	}
	if !strings.Contains(line, "=") {
		h, p := splitHostPort(line)
		return h, p, p, p
	}
	for _, segment := range strings.Split(line, ";") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		kv := strings.SplitN(segment, "=", 2)
		if len(kv) != 2 {
			continue
		}
		h, p := splitHostPort(strings.TrimSpace(kv[1]))
		switch strings.ToLower(strings.TrimSpace(kv[0])) {
		case "http":
			httpPort = p
			host = h
		case "https":
			httpsPort = p
		case "socks":
			socksPort = p
		}
	}
	return host, httpPort, httpsPort, socksPort
}

func splitHostPort(value string) (string, int) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "127.0.0.1", 0
	}
	if strings.HasPrefix(value, "[") {
		end := strings.Index(value, "]")
		if end > 0 {
			host := value[1:end]
			rest := strings.TrimPrefix(value[end+1:], ":")
			port, _ := strconv.Atoi(rest)
			return host, port
		}
	}
	if i := strings.LastIndex(value, ":"); i > 0 {
		host := value[:i]
		port, _ := strconv.Atoi(value[i+1:])
		return host, port
	}
	return value, 0
}

func winINETFromSettings(settings Settings) (winINETSnapshot, error) {
	enabled := parseBool(settings["HTTPEnable"]) || parseBool(settings["HTTPSEnable"]) || parseBool(settings["SOCKSEnable"])
	if v, ok := settings["__ProtocolEnabled"]; ok {
		enabled = parseBool(v)
	}
	s := winINETSnapshot{ProxyEnable: enabled}
	if enabled {
		port := parsePort(settings["HTTPPort"])
		if port == 0 {
			port = parsePort(settings["HTTPSPort"])
		}
		if port == 0 {
			port = parsePort(settings["SOCKSPort"])
		}
		if port < 1 || port > 65535 {
			return s, fmt.Errorf("代理端口无效")
		}
		s.ProxyServer = proxyServerLine(port)
	}
	if raw, ok := settings["ExceptionsList"]; ok {
		s.ProxyOverride = exceptionsToOverride(parseExceptions(raw))
	}
	s.AutoDiscovery = parseBool(settings["ProxyAutoDiscoveryEnable"])
	if parseBool(settings["ProxyAutoConfigEnable"]) {
		// URL itself is not modeled in Settings; disabling clears any prior URL on write.
		s.AutoConfigURL = ""
	} else {
		s.AutoConfigURL = ""
	}
	return s, nil
}
