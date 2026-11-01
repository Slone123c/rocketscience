//go:build darwin

package core

import (
	"os"
	"testing"
)

// ROCKETSCIENCE_TRACE=<ipv4> runs a real outbound trace; skipped otherwise.
func TestTraceForwardLive(t *testing.T) {
	target := os.Getenv("ROCKETSCIENCE_TRACE")
	if target == "" {
		t.Skip("set ROCKETSCIENCE_TRACE to trace a real address")
	}
	route := traceForward(target)
	if !route.Known {
		t.Fatalf("trace failed: %s", route.Error)
	}
	t.Logf("%s", route.Verdict)
	t.Logf("%v", route.Hops)
	for _, s := range route.Segments {
		t.Logf("%s (%s) %d 跳 %d ms", s.Label, s.Kind, s.Hops, s.MS)
	}
}
