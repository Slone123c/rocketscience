package systemproxy

import "encoding/json"

func settingsSubsetEqual(current, expected Settings, keys []string) bool {
	for _, k := range keys {
		a, aok := current[k]
		b, bok := expected[k]
		if aok != bok {
			return false
		}
		if aok && string(a) != string(b) {
			return false
		}
	}
	return true
}

// applyChanges returns conflicting service IDs and an updated service list.
func applyChanges(services []Service, changes []Change) ([]string, []Service) {
	out := make([]Service, len(services))
	for i, s := range services {
		clone := Settings{}
		for k, v := range s.Settings {
			clone[k] = append(json.RawMessage(nil), v...)
		}
		out[i] = Service{ID: s.ID, Name: s.Name, Settings: clone}
	}
	byID := make(map[string]int, len(out))
	for i := range out {
		byID[out[i].ID] = i
	}
	var conflicts []string
	for _, c := range changes {
		i, ok := byID[c.ID]
		if !ok {
			conflicts = append(conflicts, c.ID)
			continue
		}
		current := out[i].Settings
		checkKeys := c.Keys
		if len(c.GuardKeys) > 0 {
			checkKeys = c.GuardKeys
		}
		if !settingsSubsetEqual(current, c.Expected, checkKeys) {
			conflicts = append(conflicts, c.ID)
			continue
		}
		for _, k := range c.Keys {
			if v, ok := c.Desired[k]; ok {
				out[i].Settings[k] = append(json.RawMessage(nil), v...)
			} else {
				delete(out[i].Settings, k)
			}
		}
	}
	return conflicts, out
}
