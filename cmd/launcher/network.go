package main

import (
	"context"
	"time"

	"github.com/egoist/mygo/ui"

	egress "github.com/finch-xu/clawdhome/lima-egress"
	"github.com/finch-xu/clawdhome/vm"
)

const networkRefreshTimeout = 16 * time.Second

// routeLabel 按当前语言给出出口路由的名字。
func (a *app) routeLabel(route egress.Route) string {
	switch route {
	case egress.RouteClash:
		return "Clash"
	case egress.RouteSystem:
		return tr("route.system", a.locale)
	case egress.RouteBlocked:
		return tr("route.blocked", a.locale)
	default:
		return tr("route.unknown", a.locale)
	}
}

func (a *app) networkPanel(c *ui.Context) {
	ui.Column(c).Grow(1).MinHeight(0).Gap(6).Children(func() {
		ui.Row(c.Key("routes")).Gap(6).Children(func() {
			oldGateway := a.state == "running" && (a.network == nil || a.network.Route == "")
			for _, route := range []egress.Route{egress.RouteClash, egress.RouteSystem, egress.RouteBlocked} {
				if ui.Button(c.Key(string(route)), a.routeLabel(route)).Disabled(a.busy || oldGateway || route == a.preferredRoute).Clicked() {
					a.setRoute(route)
				}
			}
			if ui.Button(c, tr("network.refresh", a.locale)).Disabled(a.networkBusy || a.state != "running").Clicked() {
				a.refreshNetwork(false)
			}
			if ui.Button(c, tr("network.testAnthropic", a.locale)).Disabled(a.networkBusy || a.state != "running").Clicked() {
				a.refreshNetwork(true)
			}
			if ui.Button(c, tr("network.backToTerm", a.locale)).Clicked() {
				a.networkOpen = false
			}
		})
		a.connectionTable(c.Key("connection"))
		if a.networkMessage.key != "" {
			a.noticeView(c.Key("network-message"), a.networkMessage)
		}
		if a.state == "running" && a.networkText != "" {
			ui.Text(c, tr("network.snapshotTitle", a.locale))
			ui.ScrollBoth(c.Key("network-scroll")).Grow(1).MinHeight(0).Children(func() {
				ui.Text(c, a.networkText).NoWrap()
			})
		}
	})
}

type connectionRow struct {
	key, label, value, detail string
}

func (a *app) connectionRows(now time.Time) []connectionRow {
	status := vm.DescribeStatus(a.state, a.ssh, a.cfg, a.network, now)
	actual, doh := tr("network.unknownEgress", a.locale), tr("route.unknown", a.locale)
	switch {
	case a.state == vm.VMStopped || a.state == "":
		actual = tr("network.nextLaunch", a.locale) + a.routeLabel(a.preferredRoute)
		doh = tr("firewall.not-running", a.locale)
	case a.state == vm.VMRunning && a.network != nil && now.Sub(a.network.UpdatedAt) > vm.StatusExpiry:
		actual = tr("probe.expired", a.locale)
	case a.state == vm.VMRunning && a.network != nil && a.network.Running && a.network.Route != "":
		actual = a.routeLabel(a.network.Route)
		if a.network.Route == egress.RouteClash && status.CurrentProxyAddr != "" {
			actual += " · " + status.CurrentProxyAddr
		}
		if a.network.Route != a.preferredRoute {
			actual += tr("network.switchingTo", a.locale) + a.routeLabel(a.preferredRoute)
		}
		switch a.network.Route {
		case egress.RouteClash, egress.RouteSystem:
			doh = tr("network.dohPolicy", a.locale)
		case egress.RouteBlocked:
			doh = tr("network.dohBlocked", a.locale)
		}
	}
	return []connectionRow{
		{"route", tr("network.route", a.locale), actual, ""},
		{"handshake", tr("network.handshake", a.locale), tr("proxy."+status.Proxy, a.locale) + detailSuffix(a.proxyDetail(status)), status.ProxyError},
		{"probe", tr("network.probe", a.locale), tr("probe."+status.Probe, a.locale) + detailSuffix(a.probeDetail(status)), status.ProbeReason},
		{"firewall", tr("network.firewall", a.locale), tr("firewall."+status.Firewall, a.locale) + detailSuffix(a.firewallDetail(status)), ""},
		{"doh", "DoH", doh, ""},
	}
}

func (a *app) connectionTable(c *ui.Context) {
	ui.Column(c).Gap(3).Children(func() {
		for _, row := range a.connectionRows(time.Now()) {
			ui.Row(c.Key(row.key)).Gap(10).Children(func() {
				ui.Text(c, row.label).Width(112).Shrink(0).TextColor(c.Theme().TextMuted)
				value := ui.Text(c, row.value).Grow(1).MinWidth(0).Wrap()
				if row.detail != "" {
					value.Tooltip(row.detail)
				}
			})
		}
	})
}

func detailSuffix(detail string) string {
	if detail == "" {
		return ""
	}
	return "  ·  " + detail
}

func (a *app) setRoute(route egress.Route) {
	if err := a.launcher.SetRoute(route); err != nil {
		a.networkMessage = errorNotice("error.route", err)
		return
	}
	a.preferredRoute = route
	a.networkMessage = notice{}
	a.poll()
}

func (a *app) clearNetworkSnapshot() {
	a.snapshotRevision++
	a.networkBusy = false
	a.networkText, a.networkMessage = "", notice{}
}

func (a *app) refreshNetwork(anthropic bool) {
	if a.state != "running" || a.networkBusy {
		return
	}
	a.networkBusy, a.networkMessage = true, notice{key: "network.reading"}
	launcher, revision := a.launcher, a.snapshotRevision
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), networkRefreshTimeout)
		defer cancel()
		snapshot, err := launcher.NetworkSnapshot(ctx, anthropic)
		a.window.Update(func() {
			if revision != a.snapshotRevision {
				return
			}
			a.networkBusy = false
			if err != nil {
				a.networkMessage = errorNotice("error.snapshot", err)
				return
			}
			a.networkText = snapshot
			a.networkMessage = notice{key: "network.snapshotAt", args: []any{time.Now().Format("15:04:05")}}
		})
	}()
}
