//go:build windows

package systemproxy

func nativeJournalChange(s Service, port int) (Change, bool) {
	if s.ID != winINETServiceID {
		return Change{}, false
	}
	nativeKeys := []string{winINETKeyProxyServerRaw, winINETKeyAutoConfigURL}
	return Change{
		ID:       s.ID,
		Expected: pickKeys(s.Settings, nativeKeys),
		Desired: Settings{
			winINETKeyProxyServerRaw: rawString(proxyServerLine(port)),
			winINETKeyAutoConfigURL:  rawString(""),
		},
		Keys: nativeKeys,
	}, true
}
