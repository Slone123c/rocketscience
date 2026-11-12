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

func cloneSettingsMap(in Settings) Settings {
	out := Settings{}
	for k, v := range in {
		out[k] = append(json.RawMessage(nil), v...)
	}
	return out
}

// applyChanges returns conflicting service IDs and an updated service list.
// Each change is validated against the immutable pre-transaction snapshot, not
// against settings already mutated by earlier changes in the same batch.
func applyChanges(services []Service, changes []Change) ([]string, []Service) {
	out := make([]Service, len(services))
	baseline := make(map[string]Settings, len(services))
	byID := make(map[string]int, len(services))
	for i, s := range services {
		snap := cloneSettingsMap(s.Settings)
		baseline[s.ID] = snap
		out[i] = Service{ID: s.ID, Name: s.Name, Settings: cloneSettingsMap(snap)}
		byID[s.ID] = i
	}
	conflicted := map[string]bool{}
	var conflicts []string
	addConflict := func(id string) {
		if !conflicted[id] {
			conflicted[id] = true
			conflicts = append(conflicts, id)
		}
	}
	for _, c := range changes {
		i, ok := byID[c.ID]
		if !ok {
			addConflict(c.ID)
			continue
		}
		if conflicted[c.ID] {
			continue
		}
		checkKeys := c.Keys
		if len(c.GuardKeys) > 0 {
			checkKeys = c.GuardKeys
		}
		if !settingsSubsetEqual(baseline[c.ID], c.Expected, checkKeys) {
			addConflict(c.ID)
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
	for id := range conflicted {
		i := byID[id]
		out[i].Settings = cloneSettingsMap(baseline[id])
	}
	return conflicts, out
}
