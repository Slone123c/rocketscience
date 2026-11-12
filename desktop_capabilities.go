package main

import (
	"runtime"

	"rocketscience/internal/core"
)

type PlatformCapabilities struct {
	Platform       string `json:"platform"`
	SystemProxy    bool   `json:"systemProxy"`
	Tun            bool   `json:"tun"`
	Dock           bool   `json:"dock"`
	TrustedWiFi    bool   `json:"trustedWiFi"`
	RouteDiagnosis bool   `json:"routeDiagnosis"`
}

func (d *Desktop) Capabilities() PlatformCapabilities {
	cap := PlatformCapabilities{
		Platform:    runtime.GOOS,
		SystemProxy: true,
		Tun:         core.TunTakeoverAvailable(),
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
