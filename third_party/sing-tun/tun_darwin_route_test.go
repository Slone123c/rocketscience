//go:build darwin

package tun

import (
	"errors"
	"net/netip"
	"testing"

	"golang.org/x/sys/unix"
)

// fakeRoutes stands in for the routing table: destination → gateway.
type fakeRoutes map[netip.Prefix]netip.Addr

func (f fakeRoutes) install(t *testing.T) {
	t.Helper()
	add, del, get := routeAdd, routeDelete, routeGateway
	t.Cleanup(func() { routeAdd, routeDelete, routeGateway = add, del, get })
	routeAdd = func(destination netip.Prefix, gateway netip.Addr) error {
		if _, ok := f[destination]; ok {
			return unix.EEXIST
		}
		f[destination] = gateway
		return nil
	}
	routeDelete = func(destination netip.Prefix, _ netip.Addr) error {
		if _, ok := f[destination]; !ok {
			return unix.ESRCH
		}
		delete(f, destination)
		return nil
	}
	routeGateway = func(destination netip.Prefix) (netip.Addr, error) {
		gateway, ok := f[destination]
		if !ok {
			return netip.Addr{}, unix.ESRCH
		}
		return gateway, nil
	}
}

var (
	fakeIPRange = netip.MustParsePrefix("1.0.0.0/8")
	ourGateway  = netip.MustParseAddr("198.18.0.1")
)

func TestAddOwnRouteReplacesPreviousSessionRoute(t *testing.T) {
	table := fakeRoutes{fakeIPRange: ourGateway}
	table.install(t)
	if err := addOwnRoute(routeEntry{fakeIPRange, ourGateway}); err != nil {
		t.Fatalf("leftover route of our own was not replaced: %v", err)
	}
	if table[fakeIPRange] != ourGateway {
		t.Fatalf("route = %v", table[fakeIPRange])
	}
}

func TestAddOwnRouteKeepsOtherGatewayRoute(t *testing.T) {
	other := netip.MustParseAddr("10.8.0.1")
	table := fakeRoutes{fakeIPRange: other}
	table.install(t)
	err := addOwnRoute(routeEntry{fakeIPRange, ourGateway})
	if err != unix.EEXIST {
		t.Fatalf("err = %v, want EEXIST", err)
	}
	if table[fakeIPRange] != other {
		t.Fatal("deleted a route that is not ours")
	}
}

func TestCloseRemovesAutoRoutes(t *testing.T) {
	v6 := netip.MustParsePrefix("2000::/3")
	v6Gateway := netip.MustParseAddr("fdfe:dcba:9876::1")
	table := fakeRoutes{fakeIPRange: ourGateway, v6: v6Gateway}
	table.install(t)
	routes := []routeEntry{{fakeIPRange, ourGateway}, {v6, v6Gateway}, {netip.MustParsePrefix("2.0.0.0/7"), ourGateway}}
	if err := deleteRoutes(routes); err != nil {
		t.Fatalf("deleteRoutes: %v (already-gone routes must not fail)", err)
	}
	if len(table) != 0 {
		t.Fatalf("routes left after close: %v", table)
	}
}

// A stop that left our routes behind (helper killed, teardown timed out) must
// not block the next start, and the next start's routes must be complete.
func TestStartAfterStopWithStaleOwnRoutes(t *testing.T) {
	v6 := netip.MustParsePrefix("2000::/3")
	v6Gateway := netip.MustParseAddr("fdfe:dcba:9876::1")
	table := fakeRoutes{fakeIPRange: ourGateway, v6: v6Gateway}
	table.install(t)
	for _, entry := range []routeEntry{{fakeIPRange, ourGateway}, {v6, v6Gateway}} {
		if err := addOwnRoute(entry); err != nil {
			t.Fatalf("start after stale stop: %v", err)
		}
	}
	if len(table) != 2 || table[fakeIPRange] != ourGateway || table[v6] != v6Gateway {
		t.Fatalf("routes = %v", table)
	}
}

// When our leftover route cannot be deleted, the error must not look like
// another VPN's route (EEXIST is what the helper reports as "other VPN").
func TestAddOwnRouteDeleteFailureIsNotEEXIST(t *testing.T) {
	table := fakeRoutes{fakeIPRange: ourGateway}
	table.install(t)
	routeDelete = func(netip.Prefix, netip.Addr) error { return unix.EPERM }
	err := addOwnRoute(routeEntry{fakeIPRange, ourGateway})
	if err == nil || errors.Is(err, unix.EEXIST) {
		t.Fatalf("err = %v, want a non-EEXIST error", err)
	}
}
