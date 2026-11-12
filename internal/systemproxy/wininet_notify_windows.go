//go:build windows

package systemproxy

import "syscall"

var (
	modWininet                    = syscall.NewLazyDLL("wininet.dll")
	modKernel32                   = syscall.NewLazyDLL("kernel32.dll")
	procGlobalFree                = modKernel32.NewProc("GlobalFree")
	procInternetSetOptionW        = modWininet.NewProc("InternetSetOptionW")
	procInternetOpenW             = modWininet.NewProc("InternetOpenW")
	procInternetCloseHandle       = modWininet.NewProc("InternetCloseHandle")
	procInternetQueryOptionW      = modWininet.NewProc("InternetQueryOptionW")
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
