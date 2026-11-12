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

func buildProxyServerFromSettings(s Settings) string {
	var parts []string
	add := func(scheme string, enabled bool, host string, port int) {
		if !enabled || port < 1 || port > 65535 || host == "" {
			return
		}
		parts = append(parts, fmt.Sprintf("%s=%s:%d", scheme, host, port))
	}
	add("http", parseBool(s["HTTPEnable"]), parseString(s["HTTPProxy"]), parsePort(s["HTTPPort"]))
	add("https", parseBool(s["HTTPSEnable"]), parseString(s["HTTPSProxy"]), parsePort(s["HTTPSPort"]))
	add("socks", parseBool(s["SOCKSEnable"]), parseString(s["SOCKSProxy"]), parsePort(s["SOCKSPort"]))
	return strings.Join(parts, ";")
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
	ProxyEnable   bool
	ProxyServer   string
	ProxyOverride string
	AutoConfigURL string
	AutoDiscovery bool
}

func parseWinINETProxyServer(line string) (host string, httpPort, httpsPort, socksPort int) {
	ep := parseWinINETEndpointsPortable(line)
	return ep.HTTPHost, ep.HTTPPort, ep.HTTPSPort, ep.SOCKSPort
}

func parseWinINETEndpointsPortable(line string) winINETEndpoints {
	var out winINETEndpoints
	line = strings.TrimSpace(line)
	if line == "" {
		return out
	}
	if !strings.Contains(line, "=") {
		h, p := splitHostPort(line)
		out.HTTPHost, out.HTTPPort = h, p
		out.HTTPSHost, out.HTTPSPort = h, p
		out.SOCKSHost, out.SOCKSPort = h, p
		return out
	}
	for _, segment := range strings.Split(line, ";") {
		segment = strings.TrimSpace(segment)
		kv := strings.SplitN(segment, "=", 2)
		if len(kv) != 2 {
			continue
		}
		h, p := splitHostPort(strings.TrimSpace(kv[1]))
		switch strings.ToLower(strings.TrimSpace(kv[0])) {
		case "http":
			out.HTTPHost, out.HTTPPort = h, p
		case "https":
			out.HTTPSHost, out.HTTPSPort = h, p
		case "socks":
			out.SOCKSHost, out.SOCKSPort = h, p
		}
	}
	return out
}

type winINETEndpoints struct {
	HTTPHost  string
	HTTPPort  int
	HTTPSHost string
	HTTPSPort int
	SOCKSHost string
	SOCKSPort int
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

