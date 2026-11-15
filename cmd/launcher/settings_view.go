package main

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	egress "github.com/finch-xu/clawdhome/lima-egress"
	"github.com/finch-xu/clawdhome/vm"
)

// statusBar 渲染窗口底部的状态条。详情面板在状态条上方展开，这样常驻的那一行
// 始终贴着窗口下边缘。
func (a *app) statusBar(c *ui.Context) {
	status := vm.DescribeStatus(a.state, a.ssh, a.cfg, a.network, time.Now())
	if a.detailsOpen {
		ui.Column(c.Key("details")).Gap(2).Children(func() {
			for _, line := range []struct{ label, text string }{
				{"VM", status.VMDetail}, {"Clash", status.ProxyDetail},
				{"Google", status.ProbeDetail},
				{tr("status.firewall", a.locale), status.FirewallDetail},
			} {
				if line.text != "" {
					ui.Text(c.Key(line.label), line.label+"  "+line.text).SingleLine().Ellipsis("…")
				}
			}
		})
	}
	ui.Row(c.Key("summary")).Gap(6).Children(func() {
		ui.Text(c, status.Summary()).Grow(1).SingleLine().Ellipsis("…")
		label := tr("status.details", a.locale)
		if a.detailsOpen {
			label = tr("status.hideDetails", a.locale)
		}
		if ui.Button(c, label).Clicked() {
			a.detailsOpen = !a.detailsOpen
		}
		if ui.Button(c, tr("status.network", a.locale)).Clicked() {
			a.networkOpen = !a.networkOpen
			if a.networkOpen {
				a.refreshNetwork(false)
			}
		}
	})
}

func (a *app) settingsView(c *ui.Context) {
	ui.Column(c).Padding(10).Gap(6).Children(func() {
		ui.Row(c.Key("folder")).Gap(6).Children(func() {
			project := a.draft.Project
			if project == "" {
				project = tr("settings.notSelected", a.locale)
			}
			ui.Text(c, tr("settings.projectDir", a.locale)+project).Grow(1).SingleLine().Ellipsis("…")
			if ui.Button(c, tr("settings.chooseDir", a.locale)).Disabled(a.busy).Clicked() {
				a.chooseDirectory()
			}
		})
		if a.state == "running" {
			ui.Text(c, tr("settings.projectHint", a.locale))
		}
		ui.Row(c.Key("endpoint")).Gap(6).Children(func() {
			ui.Text(c, "Clash SOCKS5 / Mixed  ·  127.0.0.1")
			ui.TextInput(c, &a.draft.Port).Width(105).
				Label(tr("settings.clashPort", a.locale)).
				Placeholder(tr("settings.portHint", a.locale)).Disabled(a.busy || a.portTesting)
			if ui.Button(c, tr("settings.testPort", a.locale)).Disabled(a.busy || a.portTesting).Clicked() {
				a.testPort()
			}
			ui.Text(c, a.portTestMessage).Grow(1).SingleLine().Ellipsis("…")
		})
		a.appearanceRow(c.Key("appearance"))
		a.languageRow(c.Key("language"))
		ui.Text(c, tr("settings.langHint", a.locale))
		ui.Text(c, tr("settings.vmEnv", a.locale)+vm.GuestTimeZone)
		ui.Text(c, fmt.Sprintf(tr("settings.probePolicy", a.locale),
			int(egress.ProbeInterval.Seconds()), int(egress.ProbeTimeout.Seconds()), egress.RecoveryProbes))
		ui.Row(c.Key("actions")).Gap(6).Children(func() {
			ui.Text(c, a.settingsMessage).Grow(1).SingleLine().Ellipsis("…")
			if ui.Button(c, tr("settings.cancel", a.locale)).Clicked() {
				a.settingsOpen = false
			}
			if ui.PrimaryButton(c, tr("settings.save", a.locale)).Disabled(a.busy || a.draft == a.cfg).Clicked() {
				a.saveSettings()
			}
		})
	})
	ui.Divider(c.Key("divider"))
}

// appearanceRow 是外观三态选择：跟随系统 / 浅色 / 深色。
func (a *app) appearanceRow(c *ui.Context) {
	ui.Row(c).Gap(6).Children(func() {
		ui.Text(c, tr("settings.appearance", a.locale))
		for _, choice := range []struct {
			pref, key string
		}{
			{"", "settings.followSystem"},
			{string(mygo.ThemeLight), "settings.light"},
			{string(mygo.ThemeDark), "settings.dark"},
		} {
			selected := a.draft.ThemePref == choice.pref
			if ui.Button(c.Key("theme-"+choice.pref), tr(choice.key, a.locale)).
				Disabled(selected).Clicked() {
				a.draft.ThemePref = choice.pref
				mygo.Theme.SetSource(toThemeSource(choice.pref))
			}
		}
	})
}

// languageRow 是语言三态选择：跟随系统 / English / 简体中文。
func (a *app) languageRow(c *ui.Context) {
	ui.Row(c).Gap(6).Children(func() {
		ui.Text(c, tr("settings.language", a.locale))
		for _, choice := range []struct {
			pref, label string
		}{
			{"", tr("settings.followSystem", a.locale)},
			{string(localeEN), "English"},
			{string(localeZhHans), "简体中文"},
		} {
			selected := a.draft.LocalePref == choice.pref
			if ui.Button(c.Key("locale-"+choice.pref), choice.label).
				Disabled(selected).Clicked() {
				a.draft.LocalePref = choice.pref
				if choice.pref == "" {
					a.locale = toLocale(mygo.App.Locale())
				} else {
					mygo.App.SetLocale(choice.pref)
					a.locale = toLocale(choice.pref)
				}
			}
		}
	})
}

func (a *app) saveSettings() {
	cfg := a.draft
	cfg.Port = strings.TrimSpace(cfg.Port)
	if err := a.launcher.SaveSettings(cfg); err != nil {
		a.settingsMessage = err.Error()
		return
	}
	a.cfg, a.draft = cfg, cfg
	a.settingsMessage = tr("settings.saved", a.locale)
}

func (a *app) testPort() {
	port := strings.TrimSpace(a.draft.Port)
	if !vm.ValidPort(port) {
		a.portTestMessage = tr("settings.portRange", a.locale)
		return
	}
	a.portTesting, a.portTestMessage = true, tr("settings.portTesting", a.locale)+port+"…"
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), egress.ProbeTimeout)
		defer cancel()
		err := egress.CheckSOCKS5(ctx, net.JoinHostPort("127.0.0.1", port))
		a.window.Update(func() {
			a.portTesting = false
			if err != nil {
				a.portTestMessage = port + tr("settings.portFailed", a.locale) + err.Error()
			} else {
				a.portTestMessage = port + tr("settings.portOK", a.locale)
			}
		})
	}()
}
