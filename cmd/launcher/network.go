package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	egress "github.com/finch-xu/clawdhome/lima-egress"
)

func routeLabel(route egress.Route) string {
	switch route {
	case egress.RouteClash:
		return "Clash"
	case egress.RouteSystem:
		return "系统路由"
	case egress.RouteBlocked:
		return "断开公网"
	default:
		return "未知"
	}
}

func (l launcher) setRoute(route egress.Route) error {
	file, err := os.CreateTemp(l.home, ".route-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, err = file.WriteString(string(route) + "\n")
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), filepath.Join(l.home, egress.RouteFile))
}

func (a *app) networkPanel(c *ui.Context) {
	ui.Column(c).Grow(1).MinHeight(0).Gap(6).Children(func() {
		ui.Row(c.Key("routes")).Gap(8).Children(func() {
			oldGateway := a.state == "running" && (a.network == nil || a.network.Route == "")
			for _, route := range []egress.Route{egress.RouteClash, egress.RouteSystem, egress.RouteBlocked} {
				if ui.Button(c.Key(string(route)), routeLabel(route)).Disabled(a.busy || oldGateway || route == a.preferredRoute).Clicked() {
					if err := a.launcher.setRoute(route); err != nil {
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
		current := "当前出口 " + routeLabel(a.preferredRoute)
		if a.state == "running" {
			if a.network == nil || a.network.Route == "" {
				current = "当前出口未知 · 若 VM 正在运行旧版网关，请重启 VM"
			} else {
				current = "当前出口 " + routeLabel(a.network.Route)
				if a.network.Route != a.preferredRoute {
					current += " · 正在切换到 " + routeLabel(a.preferredRoute)
				}
			}
		} else {
			current = "下次启动使用 " + routeLabel(a.preferredRoute)
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
		ctx, cancel := context.WithTimeout(context.Background(), 16*time.Second)
		defer cancel()
		var script string
		if anthropic {
			script = `
printf '=== Anthropic DNS A ===\n'
/usr/bin/dig +time=2 +tries=1 api.anthropic.com A
printf '\n=== Anthropic HTTPS ===\n'
/usr/bin/curl --noproxy '*' --max-time 8 -sS -o /dev/null -w 'HTTP %{http_code}\n' https://api.anthropic.com/ 2>&1
printf '\n'
`
		}
		script += `
printf '=== IPv4 sockets ===\n'; /usr/sbin/netstat -anv -f inet
printf '\n=== IPv6 sockets ===\n'; /usr/sbin/netstat -anv -f inet6
printf '\n=== Interfaces ===\n'; /sbin/ifconfig -a
printf '\n=== Routes ===\n'; /usr/sbin/netstat -rn
printf '\n=== DNS resolvers ===\n'; /usr/sbin/scutil --dns
`
		out, err := commandOutput(a.launcher.cmd(ctx, "", a.launcher.lima, vmShellArgs(script)...))
		a.window.Update(func() {
			a.networkBusy = false
			if err != nil {
				a.networkMessage = fmt.Sprintf("读取失败 (%v)", err)
				return
			}
			a.networkText = strings.TrimSpace(string(out))
			a.networkMessage = "VM 网络快照 · " + time.Now().Format("15:04:05")
		})
	}()
}
