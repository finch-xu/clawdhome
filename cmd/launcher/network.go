package main

import (
	"context"
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	egress "github.com/finch-xu/clawdhome/lima-egress"
	"github.com/finch-xu/clawdhome/vm"
)

const networkRefreshTimeout = 16 * time.Second

func (a *app) networkPanel(c *ui.Context) {
	ui.Column(c).Grow(1).MinHeight(0).Gap(6).Children(func() {
		ui.Row(c.Key("routes")).Gap(8).Children(func() {
			oldGateway := a.state == "running" && (a.network == nil || a.network.Route == "")
			for _, route := range []egress.Route{egress.RouteClash, egress.RouteSystem, egress.RouteBlocked} {
				if ui.Button(c.Key(string(route)), vm.RouteLabel(route)).Disabled(a.busy || oldGateway || route == a.preferredRoute).Clicked() {
					if err := a.launcher.SetRoute(route); err != nil {
						a.networkMessage = err.Error()
					} else {
						a.preferredRoute = route
						a.networkMessage = ""
						a.poll()
					}
				}
			}
			if ui.Button(c, "刷新连接").Disabled(a.networkBusy || a.state != "running").Clicked() {
				a.refreshNetwork(false)
			}
			if ui.Button(c, "测试 Anthropic").Disabled(a.networkBusy || a.state != "running").Clicked() {
				a.refreshNetwork(true)
			}
			if ui.Button(c, "返回终端").Clicked() {
				a.networkOpen = false
			}
		})
		current := "当前出口 " + vm.RouteLabel(a.preferredRoute)
		if a.state == "running" {
			if a.network == nil || a.network.Route == "" {
				current = "当前出口未知 · 若 VM 正在运行旧版网关，请重启 VM"
			} else {
				current = "当前出口 " + vm.RouteLabel(a.network.Route)
				if a.network.Route != a.preferredRoute {
					current += " · 正在切换到 " + vm.RouteLabel(a.preferredRoute)
				}
			}
		} else {
			current = "下次启动使用 " + vm.RouteLabel(a.preferredRoute)
		}
		ui.Text(c, current)
		if a.networkMessage != "" {
			ui.Text(c, a.networkMessage)
		}
		if a.state == "running" && a.networkText != "" {
			ui.ScrollBoth(c.Key("network-scroll")).Grow(1).MinHeight(0).Children(func() {
				ui.Text(c, a.networkText).NoWrap()
			})
		}
	})
}

func (a *app) refreshNetwork(anthropic bool) {
	if a.state != "running" || a.networkBusy {
		return
	}
	a.networkBusy, a.networkMessage = true, "正在读取 VM 网络信息…"
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), networkRefreshTimeout)
		defer cancel()
		snapshot, err := a.launcher.NetworkSnapshot(ctx, anthropic)
		a.window.Update(func() {
			a.networkBusy = false
			if err != nil {
				a.networkMessage = fmt.Sprintf("读取失败 (%v)", err)
				return
			}
			a.networkText = snapshot
			a.networkMessage = "VM 网络快照 · " + time.Now().Format("15:04:05")
		})
	}()
}
