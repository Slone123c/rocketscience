package systemproxy

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

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

const gnomeServiceID = "gnome"

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
