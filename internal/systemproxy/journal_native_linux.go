//go:build linux

package systemproxy

import "encoding/json"

func nativeJournalChange(s Service, port int, bypass *[]string) (Change, bool) {
	if s.ID != gnomeServiceID {
		return Change{}, false
	}
	nativeKeys := []string{gnomeKeyMode, gnomeKeyAutoURL, gnomeKeyIgnoreHosts}
	expected := pickKeys(s.Settings, nativeKeys)
	desired := Settings{
		gnomeKeyMode:    rawString("manual"),
		gnomeKeyAutoURL: rawString(""),
	}
	if bypass != nil {
		value, err := json.Marshal(effectiveExceptions(*bypass))
		if err != nil {
			return Change{}, false
		}
		desired[gnomeKeyIgnoreHosts] = append(json.RawMessage(nil), value...)
	} else if raw, ok := s.Settings[gnomeKeyIgnoreHosts]; ok {
		desired[gnomeKeyIgnoreHosts] = append(json.RawMessage(nil), raw...)
	} else {
		desired[gnomeKeyIgnoreHosts] = rawJSON([]string(nil))
	}
	return Change{
		ID:       s.ID,
		Expected: expected,
		Desired:  desired,
		Keys:     nativeKeys,
	}, true
}
