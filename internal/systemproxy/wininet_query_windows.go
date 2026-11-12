//go:build windows

package systemproxy

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	internetOpenTypePreconfig           = 0
	internetOptionProxy                 = 38
	internetOpenTypeProxy               = 3
	internetOptionPerConnectionOption   = 75
	internetPerConnFlags                = 1
	internetPerConnProxyServer          = 2
	internetPerConnProxyBypass          = 3
	internetPerConnAutoconfigURL        = 4
	proxyTypeDirect                     = 0x00000001
	proxyTypeProxy                      = 0x00000002
	proxyTypeAutoProxyURL               = 0x00000004
	proxyTypeAutoDetect                 = 0x00000008
)

type internetProxyInfo struct {
	AccessType uint32
	Proxy      uintptr
	Bypass     uintptr
}

type internetPerConnOption struct {
	DwOption uint32
	Value    uintptr
}

type internetPerConnOptionList struct {
	Size        uint32
	_           uint32
	Connection  uintptr
	OptionCount uint32
	OptionError uint32
	Options     uintptr
}

// winINETUserSettings is what WinINET resolves for the current user connection (not raw registry alone).
type winINETUserSettings struct {
	Flags         uint32
	ProxyServer   string
	ProxyBypass   string
	AutoConfigURL string
}

func (s winINETUserSettings) ProxyEnabled() bool {
	return s.Flags&proxyTypeProxy != 0
}

func (s winINETUserSettings) AutoDetectEnabled() bool {
	return s.Flags&proxyTypeAutoDetect != 0
}

func (s winINETUserSettings) AutoProxyURLEnabled() bool {
	return s.Flags&proxyTypeAutoProxyURL != 0
}

func freeWinINETAllocatedString(ptr uintptr) {
	if ptr == 0 {
		return
	}
	procGlobalFree.Call(ptr)
}

func queryWinINETPerConnection() (winINETUserSettings, error) {
	opts := [4]internetPerConnOption{
		{DwOption: internetPerConnFlags},
		{DwOption: internetPerConnProxyServer},
		{DwOption: internetPerConnProxyBypass},
		{DwOption: internetPerConnAutoconfigURL},
	}
	list := internetPerConnOptionList{
		Size:        uint32(unsafe.Sizeof(internetPerConnOptionList{})),
		OptionCount: uint32(len(opts)),
		Options:     uintptr(unsafe.Pointer(&opts[0])),
	}
	bufSize := uint32(unsafe.Sizeof(list))
	ok, _, callErr := procInternetQueryOptionW.Call(
		0,
		internetOptionPerConnectionOption,
		uintptr(unsafe.Pointer(&list)),
		uintptr(unsafe.Pointer(&bufSize)),
	)
	if ok == 0 {
		return winINETUserSettings{}, fmt.Errorf("InternetQueryOption INTERNET_OPTION_PER_CONNECTION_OPTION: %w", callErr)
	}
	out := winINETUserSettings{Flags: uint32(opts[0].Value)}
	if opts[1].Value != 0 {
		out.ProxyServer = windows.UTF16PtrToString((*uint16)(unsafe.Pointer(opts[1].Value)))
		freeWinINETAllocatedString(opts[1].Value)
	}
	if opts[2].Value != 0 {
		out.ProxyBypass = windows.UTF16PtrToString((*uint16)(unsafe.Pointer(opts[2].Value)))
		freeWinINETAllocatedString(opts[2].Value)
	}
	if opts[3].Value != 0 {
		out.AutoConfigURL = windows.UTF16PtrToString((*uint16)(unsafe.Pointer(opts[3].Value)))
		freeWinINETAllocatedString(opts[3].Value)
	}
	return out, nil
}

func queryWinINETViaPreconfigHandle() (winINETUserSettings, error) {
	agent, _ := windows.UTF16PtrFromString("RocketScience WinINET probe")
	handle, _, callErr := procInternetOpenW.Call(
		uintptr(unsafe.Pointer(agent)),
		internetOpenTypePreconfig,
		0,
		0,
		0,
	)
	if handle == 0 {
		return winINETUserSettings{}, fmt.Errorf("InternetOpenW PRECONFIG: %w", callErr)
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
		return winINETUserSettings{}, fmt.Errorf("InternetQueryOption INTERNET_OPTION_PROXY: %w", callErr)
	}
	info := (*internetProxyInfo)(unsafe.Pointer(&buf[0]))
	out := winINETUserSettings{}
	if info.AccessType == internetOpenTypeProxy {
		out.Flags = proxyTypeProxy | proxyTypeDirect
	}
	if info.Proxy != 0 {
		out.ProxyServer = windows.UTF16PtrToString((*uint16)(unsafe.Pointer(info.Proxy)))
	}
	if info.Bypass != 0 {
		out.ProxyBypass = windows.UTF16PtrToString((*uint16)(unsafe.Pointer(info.Bypass)))
	}
	return out, nil
}

func queryWinINETUserSettings() (winINETUserSettings, error) {
	if s, err := queryWinINETPerConnection(); err == nil {
		return s, nil
	}
	return queryWinINETViaPreconfigHandle()
}

// queryWinINETProxyViaAPI reads the proxy string WinINET would use for the current user (not registry writer paths).
func queryWinINETProxyViaAPI() (proxyServer string, proxyBypass string, proxyEnabled bool, err error) {
	s, err := queryWinINETUserSettings()
	if err != nil {
		return "", "", false, err
	}
	return s.ProxyServer, s.ProxyBypass, s.ProxyEnabled(), nil
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
