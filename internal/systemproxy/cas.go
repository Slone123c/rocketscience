package systemproxy

import (
	"encoding/json"
	"fmt"
	"os"
)

func jsonScalarEqual(a, b json.RawMessage) bool {
	if string(a) == string(b) {
		return true
	}
	var sa, sb string
	if json.Unmarshal(a, &sa) == nil && json.Unmarshal(b, &sb) == nil && sa == sb {
		return true
	}
	var na, nb float64
	if json.Unmarshal(a, &na) == nil && json.Unmarshal(b, &nb) == nil && na == nb {
		return true
	}
	var ba, bb bool
	if json.Unmarshal(a, &ba) == nil && json.Unmarshal(b, &bb) == nil && ba == bb {
		return true
	}
	// Enable flags may be stored as 0/1 or false/true.
	if json.Unmarshal(a, &na) == nil && json.Unmarshal(b, &bb) == nil {
		return (na != 0) == bb
	}
	if json.Unmarshal(a, &ba) == nil && json.Unmarshal(b, &nb) == nil {
		return ba == (nb != 0)
	}
	return false
}

func jsonStringSliceEqual(a, b json.RawMessage) bool {
	var left, right []string
	if err := json.Unmarshal(a, &left); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &right); err != nil {
		return false
	}
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func settingsSubsetEqual(current, expected Settings, keys []string) bool {
	for _, k := range keys {
		a, aok := current[k]
		b, bok := expected[k]
		if aok != bok {
			return false
		}
		if !aok {
			continue
		}
		switch k {
		case "ExceptionsList", "__Gnome.IgnoreHosts":
			if !jsonStringSliceEqual(a, b) {
				return false
			}
		default:
			if !jsonScalarEqual(a, b) {
				return false
			}
		}
	}
	return true
}

func cloneSettingsMap(in Settings) Settings {
	out := Settings{}
	for k, v := range in {
		out[k] = append(json.RawMessage(nil), v...)
	}
	return out
}

// applyChanges validates each change against the immutable pre-transaction
// snapshot. Matching groups merge into updated; conflicting groups are skipped
// without blocking other groups on the same service.
func applyChanges(services []Service, changes []Change) (conflicts []string, updated []Service, applied []Change) {
	out := make([]Service, len(services))
	baseline := make(map[string]Settings, len(services))
	byID := make(map[string]int, len(services))
	for i, s := range services {
		snap := cloneSettingsMap(s.Settings)
		baseline[s.ID] = snap
		out[i] = Service{ID: s.ID, Name: s.Name, Settings: cloneSettingsMap(snap)}
		byID[s.ID] = i
	}
	seenConflict := map[string]bool{}
	for _, c := range changes {
		i, ok := byID[c.ID]
		if !ok {
			if !seenConflict[c.ID] {
				seenConflict[c.ID] = true
				conflicts = append(conflicts, c.ID)
			}
			continue
		}
		checkKeys := c.Keys
		if len(c.GuardKeys) > 0 {
			checkKeys = c.GuardKeys
		}
		if !settingsSubsetEqual(baseline[c.ID], c.Expected, checkKeys) {
			if os.Getenv("SYSTEMPROXY_DEBUG_CAS") != "" {
				for _, k := range checkKeys {
					a, aok := baseline[c.ID][k]
					b, bok := c.Expected[k]
					if aok != bok || (aok && !jsonScalarEqual(a, b) && (k != "ExceptionsList" && k != "__Gnome.IgnoreHosts" || !jsonStringSliceEqual(a, b))) {
						fmt.Fprintf(os.Stderr, "CAS conflict service=%s key=%s have=%s want=%s\n", c.ID, k, a, b)
					}
				}
			}
			if !seenConflict[c.ID] {
				seenConflict[c.ID] = true
				conflicts = append(conflicts, c.ID)
			}
			continue
		}
		for _, k := range c.Keys {
			if v, ok := c.Desired[k]; ok {
				out[i].Settings[k] = append(json.RawMessage(nil), v...)
			} else {
				delete(out[i].Settings, k)
			}
		}
		applied = append(applied, c)
	}
	return conflicts, out, applied
}
