package main

import (
	"runtime"
	"testing"
)

func TestDesktopCapabilitiesMatchPlatform(t *testing.T) {
	caps := (&Desktop{}).Capabilities()
	if caps.Platform != runtime.GOOS {
		t.Fatalf("Platform=%q want %q", caps.Platform, runtime.GOOS)
	}
	if !caps.ManualProxyAvailable {
		t.Fatal("ManualProxyAvailable must be true on all platforms")
	}
	if runtime.GOOS == "darwin" {
		if !caps.Tun || !caps.Dock || !caps.TrustedWiFi || !caps.RouteDiagnosis {
			t.Fatalf("darwin capabilities missing expected features: %+v", caps)
		}
		return
	}
	if caps.Tun || caps.Dock || caps.TrustedWiFi || caps.RouteDiagnosis {
		t.Fatalf("non-darwin capabilities must disable macOS-only features: %+v", caps)
	}
}
