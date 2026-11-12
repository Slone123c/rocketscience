package main

import (
	"runtime"

	"rocketscience/internal/core"
	"rocketscience/internal/systemproxy"
)

type PlatformCapabilities struct {
	Platform                string `json:"platform"`
	SystemProxy             bool   `json:"systemProxy"`
	SystemProxyAutomatic    bool   `json:"systemProxyAutomatic"`
	ManualProxyAvailable    bool   `json:"manualProxyAvailable"`
	ManualProxyNote         string `json:"manualProxyNote,omitempty"`
	Tun                     bool   `json:"tun"`
	Dock                    bool   `json:"dock"`
	TrustedWiFi             bool   `json:"trustedWiFi"`
	RouteDiagnosis          bool   `json:"routeDiagnosis"`
	TunFirstReleaseScope    string `json:"tunFirstReleaseScope,omitempty"`
}

func (d *Desktop) Capabilities() PlatformCapabilities {
	automatic := systemproxy.AutomaticAvailable()
	cap := PlatformCapabilities{
		Platform:               runtime.GOOS,
		SystemProxy:            automatic,
		SystemProxyAutomatic:   automatic,
		ManualProxyNote:        systemproxy.ManualFallbackNote(),
		Tun:                    core.TunTakeoverAvailable(),
		ManualProxyAvailable:   true,
		TunFirstReleaseScope:   "macOS",
	}
	switch runtime.GOOS {
	case "darwin":
		cap.Dock = true
		cap.TrustedWiFi = true
		cap.RouteDiagnosis = true
	default:
		cap.Dock = false
		cap.TrustedWiFi = false
		cap.RouteDiagnosis = false
	}
	return cap
}
