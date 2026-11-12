//go:build windows

package systemproxy

import "syscall"

var (
	modWininet                    = syscall.NewLazyDLL("wininet.dll")
	procInternetSetOptionW        = modWininet.NewProc("InternetSetOptionW")
	internetOptionSettingsChanged = uintptr(39)
	internetOptionRefresh         = uintptr(37)
)

func notifyWinINET() error {
	for _, opt := range []uintptr{internetOptionSettingsChanged, internetOptionRefresh} {
		r, _, err := procInternetSetOptionW.Call(0, opt, 0, 0)
		if r == 0 {
			return err
		}
	}
	return nil
}
