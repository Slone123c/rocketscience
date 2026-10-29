package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLinkedDNSRemainsValidWithoutTUN(t *testing.T) {
	if os.Getenv("ROCKETSCIENCE_MIHOMO") == "" {
		t.Skip("real core validation")
	}
	enabled := true
	data, err := runtimeConfigWithOptions(direct, 17890, 19090, "test", "", LANSettings{}, false, CoreOptions{DNS: &DNSOptions{Enabled: &enabled}})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := validateRuntimeWithContext(ctx, corePath(), dir, path); err != nil {
		t.Fatal(err)
	}
}
