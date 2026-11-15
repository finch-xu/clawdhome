package main

import (
	"testing"

	egress "github.com/finch-xu/clawdhome/lima-egress"
)

func TestRefreshNetworkSerializesRequestsAndClearsBusy(t *testing.T) {
	for _, script := range []string{"echo snapshot", "exit 1"} {
		t.Run(script, func(t *testing.T) {
			q := make(queuedUpdater, 4)
			a := &app{window: q, state: "running", launcher: fakeLauncher(t, script)}
			a.refreshNetwork(false)
			a.refreshNetwork(true)
			if !a.networkBusy {
				t.Fatal("refresh should be busy until the UI applies its result")
			}
			q.next(t)()
			if a.networkBusy || a.networkMessage == "" || (a.networkText == "snapshot") != (script == "echo snapshot") {
				t.Fatalf("busy=%v, text=%q, message=%q", a.networkBusy, a.networkText, a.networkMessage)
			}
		})
	}
}

func TestSetRouteKeepsActualSnapshotUntilGatewayAcknowledges(t *testing.T) {
	a := &app{busy: true, preferredRoute: egress.RouteSystem,
		launcher: fakeLauncher(t, ""), network: &egress.GatewayStatus{Route: egress.RouteSystem}}
	a.setRoute(egress.RouteBlocked)
	if a.preferredRoute != egress.RouteBlocked || a.network.Route != egress.RouteSystem {
		t.Fatalf("desired=%s, actual=%s", a.preferredRoute, a.network.Route)
	}
	a.launcher.Home += "/missing"
	a.setRoute(egress.RouteClash)
	if a.preferredRoute != egress.RouteBlocked || a.networkMessage == "" {
		t.Fatalf("desired=%s, message=%q", a.preferredRoute, a.networkMessage)
	}
}
