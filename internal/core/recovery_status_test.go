package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rocketscience/internal/systemproxy"
)

func TestRecoveryRecordErrorVisibleWhileStopped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"changes":[{"id":"wifi","keys":["HTTPPort"],"expected":{},"desired":{"HTTPPort":"invalid"}}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	s := New()
	// No backend is needed: reporting a damaged journal must never invoke one.
	s.proxy = systemproxy.New(nil, path)
	s.proxyWarning = "existing warning"
	st := s.Status()
	if st.Running || st.RecoveryPending || !strings.Contains(st.Warning, "existing warning") || !strings.Contains(st.Warning, "恢复记录") {
		t.Fatalf("recovery error hidden: %+v", st)
	}
}
