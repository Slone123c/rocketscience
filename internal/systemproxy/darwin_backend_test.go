//go:build darwin

package systemproxy

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

type readOnlyDarwin struct {
	DarwinBackend
	changes []Change
}

func (b *readOnlyDarwin) CompareAndSwap(changes []Change) ([]string, error) {
	b.changes = changes
	return nil, errors.New("read-only validation: no system writes")
}

// Explicit opt-in. Only snapshot is executed against the real helper; all write
// requests are intercepted before the native authorization entry point.
func TestDarwinReadOnlySnapshotAndRecoveryRecord(t *testing.T) {
	helper := os.Getenv("ROCKETSCIENCE_PROXY_HELPER")
	if helper == "" {
		t.Skip("set ROCKETSCIENCE_PROXY_HELPER for read-only system integration")
	}
	backend := &readOnlyDarwin{DarwinBackend: DarwinBackend{Helper: helper}}
	services, err := backend.Snapshot()
	if err != nil || len(services) == 0 {
		t.Fatalf("snapshot: %d services, %v", len(services), err)
	}
	manager := New(backend, filepath.Join(t.TempDir(), "recovery.json"))
	if err := manager.Apply(7890); !errors.Is(err, ErrRecoveryNeeded) {
		t.Fatalf("write interception failed: %v", err)
	}
	if len(backend.changes) == 0 {
		t.Fatal("no changes captured")
	}
	if pending, err := manager.Pending(); err != nil || !pending {
		t.Fatalf("real snapshot produced unreadable recovery record: %v", err)
	}
	for _, args := range [][]string{{"invalid-mode"}, {"cas"}} {
		cmd := exec.Command(helper, args...)
		if err := cmd.Run(); err == nil {
			t.Fatal("invalid request accepted")
		}
	}
	t.Logf("read %d services; intercepted every write before authorization", len(services))
}
