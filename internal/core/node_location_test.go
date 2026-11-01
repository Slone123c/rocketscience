package core

import "testing"

func TestNodeLocationsNeedRunningCore(t *testing.T) {
	service := newTestService(t)
	if got := service.NodeLocations([]string{"香港 01"}); len(got) != 0 {
		t.Fatalf("NodeLocations while stopped = %v, want empty", got)
	}
	nodeLocations.Lock()
	defer nodeLocations.Unlock()
	if _, ok := nodeLocations.cache["香港 01"]; ok {
		t.Fatal("a stopped core must not start lookups")
	}
}
