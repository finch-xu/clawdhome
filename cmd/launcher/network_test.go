package main

import (
	"strings"
	"testing"
	"time"

	"github.com/finch-xu/clawdhome/vm"

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
			if a.networkBusy || a.networkMessage.text(a.locale) == "" || (a.networkText == "snapshot") != (script == "echo snapshot") {
				t.Fatalf("busy=%v, text=%q, message=%q", a.networkBusy, a.networkText, a.networkMessage.text(a.locale))
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
	if a.preferredRoute != egress.RouteBlocked || a.networkMessage.text(a.locale) == "" {
		t.Fatalf("desired=%s, message=%q", a.preferredRoute, a.networkMessage.text(a.locale))
	}
}

func TestConnectionRowsSeparateActualRouteFromPendingAndExpiredState(t *testing.T) {
	now := time.Unix(1000, 0)
	a := &app{locale: localeEN, state: "running", preferredRoute: egress.RouteBlocked,
		network: &egress.GatewayStatus{Running: true, Route: egress.RouteClash,
			ProxyAddr: "127.0.0.1:7890", Proxy: egress.ProxyConnected, Probe: egress.ProbeUnreachable,
			UpdatedAt: now}}
	rows := a.connectionRows(now)
	if len(rows) != 5 || !strings.Contains(rows[0].value, "127.0.0.1:7890") ||
		!strings.Contains(rows[0].value, "switching to") ||
		!strings.HasPrefix(rows[1].value, "Connected") || !strings.HasPrefix(rows[2].value, "Unreachable") {
		t.Fatalf("connection rows: %+v", rows)
	}
	rows = a.connectionRows(now.Add(vm.StatusExpiry + time.Second))
	if strings.Contains(rows[0].value, "127.0.0.1") || !strings.Contains(rows[0].value, "Expired") {
		t.Fatalf("stale route appears current: %+v", rows[0])
	}
	a.state = "stopped"
	if got := a.connectionRows(now)[0].value; !strings.Contains(got, "Next launch") {
		t.Fatalf("stopped route = %q", got)
	}
	a.state, a.network = "running", nil
	if got := a.connectionRows(now)[0].value; !strings.Contains(got, "unknown") {
		t.Fatalf("missing route = %q", got)
	}
	a.network = &egress.GatewayStatus{Running: true, Route: egress.RouteSystem, UpdatedAt: now}
	a.preferredRoute, a.locale = egress.RouteSystem, localeZhHans
	if got := a.connectionRows(now); !strings.Contains(got[0].value, "系统") || !strings.Contains(got[4].value, "Cloudflare") {
		t.Fatalf("system route: %+v", got)
	}
	a.network.Route, a.preferredRoute = egress.RouteBlocked, egress.RouteBlocked
	if got := a.connectionRows(now); strings.Contains(got[4].value, "Cloudflare") {
		t.Fatalf("blocked route shows an active DoH policy: %+v", got)
	}
}
