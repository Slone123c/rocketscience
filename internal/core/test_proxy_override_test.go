package core

import (
	"testing"

	"rocketscience/internal/systemproxy"
)

func withAutomaticSystemProxy(t *testing.T) {
	t.Helper()
	systemproxy.SetAutomaticAvailableOverride(func() (bool, bool) { return true, true })
	t.Cleanup(func() { systemproxy.SetAutomaticAvailableOverride(nil) })
}
