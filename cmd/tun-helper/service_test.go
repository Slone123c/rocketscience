//go:build darwin

package main

import (
	"strings"
	"testing"

	"rocketscience/internal/tunhelper"
)

func TestPlistRunsInstalledCopyForOneUser(t *testing.T) {
	text := string(plist(501))
	for _, want := range []string{tunhelper.ServiceLabel, "<string>" + tunhelper.ServicePath + "</string>", "<string>serve</string>", "<string>501</string>", "<key>KeepAlive</key>"} {
		if !strings.Contains(text, want) {
			t.Errorf("plist misses %q", want)
		}
	}
}
