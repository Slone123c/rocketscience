//go:build windows

package systemproxy

func nativeJournalChanges(s Service, port int, _ *[]string) []Change {
	if s.ID != winINETServiceID {
		return nil
	}
	return []Change{
		{
			ID:       s.ID,
			Expected: pickKeys(s.Settings, []string{winINETKeyProxyServerRaw}),
			Desired:  Settings{winINETKeyProxyServerRaw: rawString(proxyServerLine(port))},
			Keys:     []string{winINETKeyProxyServerRaw},
		},
		{
			ID:       s.ID,
			Expected: pickKeys(s.Settings, []string{winINETKeyAutoConfigURL}),
			Desired:  Settings{winINETKeyAutoConfigURL: rawString("")},
			Keys:     []string{winINETKeyAutoConfigURL},
		},
	}
}
