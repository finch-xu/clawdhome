package main

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	egress "github.com/finch-xu/clawdhome/lima-egress"
)

func (a *app) statusBar(c *ui.Context) {
	status := describeStatus(a.state, a.ssh, a.cfg, a.network, time.Now())
	ui.Row(c.Key("summary")).Gap(8).Children(func() {
		ui.Text(c, status.summary()).Grow(1).SingleLine().Ellipsis("…")
		label := "状态详情"
		if a.detailsOpen {
			label = "收起详情"
		}
		if ui.Button(c, label).Clicked() {
			a.detailsOpen = !a.detailsOpen
		}
		if ui.Button(c, "网络连接").Clicked() {
			a.networkOpen = !a.networkOpen
			if a.networkOpen {
				a.refreshNetwork(false)
			}
		}
	})
	if a.detailsOpen {
		ui.Column(c.Key("details")).Gap(3).Children(func() {
			for _, line := range []struct{ label, text string }{
				{"VM", status.vmDetail}, {"Clash", status.proxyDetail},
				{"Google", status.probeDetail}, {"防火墙", status.firewallDetail},
			} {
				if line.text != "" {
					ui.Text(c.Key(line.label), line.label+"  "+line.text).SingleLine().Ellipsis("…")
				}
			}
		})
	}
}

func (a *app) settingsView(c *ui.Context) {
	ui.Column(c).Padding(10).Gap(6).Children(func() {
		ui.Row(c.Key("folder")).Gap(8).Children(func() {
			project := a.draft.Project
			if project == "" {
				project = "未选择"
			}
			ui.Text(c, "项目目录  "+project).Grow(1).SingleLine().Ellipsis("…")
			if ui.Button(c, "选择目录").Disabled(a.busy).Clicked() {
				a.chooseDirectory()
			}
		})
		if a.state == "running" {
			ui.Text(c, "新会话使用选定项目。端口改动在下次启动 VM 时生效。")
		}
		ui.Row(c.Key("endpoint")).Gap(8).Children(func() {
			ui.Text(c, "Clash SOCKS5 / Mixed  ·  127.0.0.1")
			ui.TextInput(c, &a.draft.Port).Width(105).Label("Clash 端口").Placeholder("端口").Disabled(a.busy || a.portTesting)
			if ui.Button(c, "测试端口").Disabled(a.busy || a.portTesting).Clicked() {
				a.testPort()
			}
			ui.Text(c, a.portTestMessage).Grow(1).SingleLine().Ellipsis("…")
		})
		ui.Text(c, "VM 环境  美国英语 · 美国地区 · "+guestTimeZone)
		ui.Text(c, fmt.Sprintf("探测策略  通常每 %d 秒 · %d 秒超时 · 连续 %d 次成功后恢复 · 429 时退避",
			int(egress.ProbeInterval.Seconds()), int(egress.ProbeTimeout.Seconds()), egress.RecoveryProbes))
		ui.Row(c.Key("actions")).Gap(8).Children(func() {
			ui.Text(c, a.settingsMessage).Grow(1).SingleLine().Ellipsis("…")
			if ui.Button(c, "取消").Clicked() {
				a.settingsOpen = false
			}
			if ui.PrimaryButton(c, "保存设置").Disabled(a.busy || a.draft == a.cfg).Clicked() {
				a.saveSettings()
			}
		})
	})
	ui.Divider(c.Key("divider"))
}

func (a *app) saveSettings() {
	cfg := a.draft
	cfg.Port = strings.TrimSpace(cfg.Port)
	if err := a.launcher.saveSettings(cfg); err != nil {
		a.settingsMessage = err.Error()
		return
	}
	a.cfg, a.draft = cfg, cfg
	a.settingsMessage = "设置已保存"
}

func (a *app) testPort() {
	port := strings.TrimSpace(a.draft.Port)
	if !validPort(port) {
		a.portTestMessage = "请输入 1–65535 之间的端口"
		return
	}
	a.portTesting, a.portTestMessage = true, "正在测试端口 "+port+"…"
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), egress.ProbeTimeout)
		defer cancel()
		err := egress.CheckSOCKS5(ctx, net.JoinHostPort("127.0.0.1", port))
		a.window.Update(func() {
			a.portTesting = false
			if err != nil {
				a.portTestMessage = port + " SOCKS5 握手失败 · " + err.Error()
			} else {
				a.portTestMessage = port + " SOCKS5 握手成功"
			}
		})
	}()
}
