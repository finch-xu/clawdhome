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

func (a *app) statusBar(c *ui.Context) {
	status := vm.DescribeStatus(a.state, a.ssh, a.cfg, a.network, time.Now())
	ui.Divider(c)
	ui.Column(c.Key("status-bar")).Background(sidebarColor(c.Theme())).Children(func() {
		if a.detailsOpen {
			ui.Column(c.Key("details")).Padding(7, 12).Gap(3).Children(func() {
				for _, row := range []connectionRow{
					{"ssh", "VM", tr("status.localSSH", a.locale) + tr("ssh."+status.SSH, a.locale), ""},
					{"proxy", "Clash", a.proxyDetail(status), status.ProxyError},
					{"probe", "Google", a.probeDetail(status), status.ProbeReason},
					{"firewall", tr("status.firewall", a.locale), a.firewallDetail(status), ""},
				} {
					if row.value != "" {
						diagnosticRow(c.Key(row.key), row)
					}
				}
			})
			ui.Divider(c)
		}
		ui.Row(c.Key("summary")).Padding(6, 10).Gap(8).Children(func() {
			ui.Row(c.Key("items")).Grow(1).MinWidth(0).Wrap().Gap(8).Children(func() {
				for _, item := range a.statusItems(status) {
					ui.Row(c.Key(item.key)).Gap(4).Children(func() {
						statusDot(c, statusColor(c.Theme(), item.tone))
						ui.Text(c, item.label).FontSize(11).SingleLine()
					})
				}
			})
			label := tr("status.details", a.locale)
			if a.detailsOpen {
				label = tr("status.hideDetails", a.locale)
			}
			if ui.Button(c.Key("show-details"), label).Clicked() {
				a.detailsOpen = !a.detailsOpen
			}
		})
	})
}

func (a *app) settingsView(c *ui.Context) {
	ui.Column(c).Grow(1).MinHeight(0).Children(func() {
		ui.Scroll(c.Key("settings-scroll")).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).Padding(16).Gap(14).Children(func() {
				a.settingsSection(c.Key("project"), "settings.projectTitle", func() {
					ui.Row(c).Gap(8).Children(func() {
						project := a.draft.Project
						if project == "" {
							project = tr("settings.notSelected", a.locale)
						}
						ui.Text(c, project).Grow(1).MinWidth(0).SingleLine().Ellipsis("…").Tooltip(project)
						if ui.Button(c, tr("settings.chooseDir", a.locale)).Disabled(a.busy).Clicked() {
							a.chooseDirectory()
						}
					})
					ui.Text(c, tr("settings.projectHint", a.locale)).TextColor(c.Theme().TextMuted)
				})
				a.settingsSection(c.Key("proxy"), "settings.proxyTitle", func() {
					ui.Row(c).Gap(8).Wrap().Children(func() {
						ui.Text(c, "127.0.0.1:")
						ui.TextInput(c.Key("clash-port"), &a.draft.Port).Width(80).
							Label(tr("settings.clashPort", a.locale)).
							Placeholder(tr("settings.portHint", a.locale)).Disabled(a.busy || a.portTesting)
						if ui.Button(c, tr("settings.testPort", a.locale)).Disabled(a.busy || a.portTesting).Clicked() {
							a.testPort()
						}
					})
					if a.portTestMessage.key != "" {
						a.noticeView(c.Key("port-message"), a.portTestMessage)
					}
					ui.Text(c, tr("settings.portRestart", a.locale)).TextColor(c.Theme().TextMuted)
				})
				a.settingsSection(c.Key("appearance"), "settings.appearanceTitle", func() {
					a.appearanceRow(c.Key("theme"))
					a.languageRow(c.Key("language"))
					ui.Text(c, tr("settings.langHint", a.locale)).TextColor(c.Theme().TextMuted)
				})
				a.settingsSection(c.Key("guest"), "settings.vmEnvTitle", func() {
					diagnosticRow(c.Key("guest-locale"), connectionRow{label: tr("settings.guestLocale", a.locale), value: "en-US · en_US"})
					diagnosticRow(c.Key("guest-zone"), connectionRow{label: tr("settings.guestZone", a.locale), value: vm.GuestTimeZone})
					ui.Text(c, fmt.Sprintf(tr("settings.probePolicy", a.locale),
						int(egress.ProbeInterval.Seconds()), int(egress.ProbeTimeout.Seconds()), egress.RecoveryProbes))
				})
			})
		})
		ui.Divider(c)
		ui.Row(c.Key("settings-actions")).Padding(8, 12).Gap(6).Children(func() {
			a.noticeView(c.Key("settings-message"), a.settingsMessage).Grow(1).MinWidth(0)
			if ui.Button(c, tr("settings.cancel", a.locale)).Clicked() {
				a.cancelSettings()
			}
			if ui.PrimaryButton(c, tr("settings.save", a.locale)).Disabled(a.busy || a.draft == a.cfg).Clicked() {
				a.saveSettings()
			}
		})
	})
}

func (a *app) settingsSection(c *ui.Context, title string, content func()) {
	ui.Column(c).Gap(6).Children(func() {
		ui.Text(c, tr(title, a.locale)).Bold()
		ui.Column(c).Gap(6).Padding(10).Radius(7).Border(1, c.Theme().Border).Children(content)
	})
}

func (a *app) appearanceRow(c *ui.Context) {
	ui.Row(c).Gap(6).Wrap().Children(func() {
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
				Selected(selected).Disabled(a.busy || selected).Clicked() {
				a.draft.ThemePref = choice.pref
				a.appearance.SetTheme(choice.pref)
			}
		}
	})
}

// languageRow 是语言三态选择：跟随系统 / English / 简体中文。
func (a *app) languageRow(c *ui.Context) {
	ui.Row(c).Gap(6).Wrap().Children(func() {
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
				Selected(selected).Disabled(a.busy || selected).Clicked() {
				a.draft.LocalePref = choice.pref
				a.applyLocalePref(choice.pref)
			}
		}
	})
}

func (a *app) saveSettings() {
	cfg := a.draft
	cfg.Port = strings.TrimSpace(cfg.Port)
	if err := a.launcher.SaveSettings(cfg); err != nil {
		a.settingsMessage = errorNotice("error.save-settings", err)
		return
	}
	a.cfg, a.draft = cfg, cfg
	a.settingsMessage = notice{key: "settings.saved"}
}

func (a *app) testPort() {
	port := strings.TrimSpace(a.draft.Port)
	if !vm.ValidPort(port) {
		a.portTestMessage = notice{key: "settings.portRange"}
		return
	}
	a.portTesting, a.portTestMessage = true, notice{key: "settings.portTesting", args: []any{port}}
	revision := a.settingsRevision
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), egress.ProbeTimeout)
		defer cancel()
		err := egress.CheckSOCKS5(ctx, net.JoinHostPort("127.0.0.1", port))
		a.window.Update(func() {
			if revision != a.settingsRevision {
				return
			}
			a.portTesting = false
			if err != nil {
				a.portTestMessage = notice{key: "settings.portFailed", args: []any{port}, detail: err.Error()}
			} else {
				a.portTestMessage = notice{key: "settings.portOK", args: []any{port}}
			}
		})
	}()
}

// applyLocalePref 立刻套用语言偏好。空串清掉 MyGo 存的覆盖，回到系统语言。
func (a *app) applyLocalePref(pref string) {
	tag := localePrefTag(pref)
	a.appearance.SetLocale(tag)
	if tag == "" {
		a.setLocale(toLocale(a.appearance.SystemLocale()))
		return
	}
	a.setLocale(toLocale(tag))
}

// cancelSettings 关掉设置面板，并把点一下就生效的外观和语言退回已保存的值，
// 否则取消之后界面会和 launcher.json 不一致。
func (a *app) cancelSettings() { a.openPage(pageSessions) }

func (a *app) discardSettingsDraft() {
	a.settingsRevision++
	a.portTesting = false
	a.settingsMessage, a.portTestMessage = notice{}, notice{}
	if a.draft.ThemePref != a.cfg.ThemePref {
		a.appearance.SetTheme(a.cfg.ThemePref)
	}
	if a.draft.LocalePref != a.cfg.LocalePref {
		a.applyLocalePref(a.cfg.LocalePref)
	}
	a.draft = a.cfg
}
