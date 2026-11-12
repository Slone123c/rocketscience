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
