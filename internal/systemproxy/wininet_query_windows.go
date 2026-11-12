//go:build windows

package systemproxy

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	internetOpenTypeDirect = 1
	internetOptionProxy    = 38
	internetOpenTypeProxy  = 3
)

type internetProxyInfo struct {
	AccessType uint32
	Proxy      uintptr
	Bypass     uintptr
}

// queryWinINETProxyViaAPI reads the current-user proxy string WinINET would consult (not the registry writer).
func queryWinINETProxyViaAPI() (proxyServer string, proxyBypass string, proxyEnabled bool, err error) {
	agent, _ := windows.UTF16PtrFromString("RocketScience WinINET probe")
	handle, _, callErr := procInternetOpenW.Call(
		uintptr(unsafe.Pointer(agent)),
		internetOpenTypeDirect,
		0,
		0,
		0,
	)
	if handle == 0 {
		return "", "", false, fmt.Errorf("InternetOpenW: %w", callErr)
	}
	defer procInternetCloseHandle.Call(handle)

	buf := make([]byte, 8192)
	size := uint32(len(buf))
	ok, _, callErr := procInternetQueryOptionW.Call(
		handle,
		internetOptionProxy,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ok == 0 {
		return "", "", false, fmt.Errorf("InternetQueryOption INTERNET_OPTION_PROXY: %w", callErr)
	}
	info := (*internetProxyInfo)(unsafe.Pointer(&buf[0]))
	proxyEnabled = info.AccessType == internetOpenTypeProxy
	if info.Proxy != 0 {
		proxyServer = windows.UTF16PtrToString((*uint16)(unsafe.Pointer(info.Proxy)))
	}
	if info.Bypass != 0 {
		proxyBypass = windows.UTF16PtrToString((*uint16)(unsafe.Pointer(info.Bypass)))
	}
	return proxyServer, proxyBypass, proxyEnabled, nil
}

// readWinINETRegistryDword opens HKCU Internet Settings and reads a DWORD (separate from WindowsBackend.Write).
func readWinINETRegistryDword(name string) (uint32, bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return 0, false, err
	}
	defer key.Close()
	data, _, err := key.GetIntegerValue(name)
	if err != nil {
		return 0, false, err
	}
	return uint32(data), true, nil
}
