//go:build linux

package systemproxy

import "encoding/json"

func nativeJournalChanges(s Service, port int, bypass *[]string) []Change {
	if s.ID != gnomeServiceID {
		return nil
	}
	ignoreDesired := Settings{}
	if bypass != nil {
		value, err := json.Marshal(effectiveExceptions(*bypass))
		if err != nil {
			return nil
		}
		ignoreDesired[gnomeKeyIgnoreHosts] = append(json.RawMessage(nil), value...)
	} else {
		entries := parseExceptions(s.Settings["ExceptionsList"])
		if len(entries) == 0 {
			if raw, ok := s.Settings[gnomeKeyIgnoreHosts]; ok {
				entries = parseExceptions(raw)
			}
		}
		value, err := json.Marshal(effectiveExceptions(entries))
		if err != nil {
			return nil
		}
		ignoreDesired[gnomeKeyIgnoreHosts] = append(json.RawMessage(nil), value...)
	}
	return []Change{
		{
			ID:       s.ID,
			Expected: pickKeys(s.Settings, []string{gnomeKeyMode}),
			Desired:  Settings{gnomeKeyMode: rawString("manual")},
			Keys:     []string{gnomeKeyMode},
		},
		{
			ID:       s.ID,
			Expected: pickKeys(s.Settings, []string{gnomeKeyAutoURL}),
			Desired:  Settings{gnomeKeyAutoURL: rawString("")},
			Keys:     []string{gnomeKeyAutoURL},
		},
		{
			ID:       s.ID,
			Expected: pickKeys(s.Settings, []string{gnomeKeyIgnoreHosts}),
			Desired:  ignoreDesired,
			Keys:     []string{gnomeKeyIgnoreHosts},
		},
	}
}
