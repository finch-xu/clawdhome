package main

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/finch-xu/clawdhome/vm"
)

type launcherPage uint8

const (
	pageSessions launcherPage = iota
	pageNetwork
	pageSettings
)

func (a *app) openPage(page launcherPage) {
	if a.page == page {
		return
	}
	if a.page == pageSettings {
		a.discardSettingsDraft()
	}
	a.page = page
	switch page {
	case pageSettings:
		a.draft = a.cfg
		a.draft.LocalePref = localePrefTag(a.cfg.LocalePref)
		a.draft.ThemePref = themePreference(a.cfg.ThemePref)
	case pageNetwork:
		a.refreshNetwork(false)
	}
}

func (a *app) view(c *ui.Context) {
	c.After(viewRefreshInterval)
	if time.Since(a.lastPoll) >= pollInterval {
		a.poll()
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		a.sidebar(c.Key("navigation"))
		ui.Divider(c)
		ui.Column(c.Key("workspace")).Grow(1).MinWidth(0).MinHeight(0).Children(func() {
			a.toolbar(c.Key("toolbar"))
			ui.Divider(c)
			if a.message.key != "" {
				ui.Box(c.Key("message")).Padding(6, 12).Children(func() {
					a.noticeView(c, a.message)
				})
			}
			switch a.page {
			case pageNetwork:
				a.networkPanel(c.Key("network"))
			case pageSettings:
				a.settingsView(c.Key("settings"))
			default:
				a.sessionsView(c.Key("sessions"))
			}
			a.statusBar(c.Key("status"))
		})
	})
}

func (a *app) sidebar(c *ui.Context) {
	ui.Column(c).Width(102).Shrink(0).Padding(8).Gap(4).Background(sidebarColor(c.Theme())).Children(func() {
		for _, item := range []struct {
			page launcherPage
			key  string
		}{{pageSessions, "nav.sessions"}, {pageNetwork, "nav.network"}, {pageSettings, "settings.title"}} {
			button := ui.Button(c.Key(item.key), tr(item.key, a.locale)).FillWidth().Selected(a.page == item.page)
			if button.Clicked() {
				a.openPage(item.page)
			}
		}
		ui.Spacer(c)
		pref := a.cfg.ThemePref
		if a.page == pageSettings {
			pref = a.draft.ThemePref
		}
		if ui.Button(c.Key("theme-shortcut"), tr(themeLabel(pref), a.locale)).
			Tooltip(tr("theme.switch", a.locale)).Disabled(a.busy).Clicked() {
			a.cycleTheme(pref)
		}
	})
}

func (a *app) toolbar(c *ui.Context) {
	ui.Row(c).Gap(8).Padding(8, 12).Children(func() {
		project := a.cfg.Project
		if project == "" {
			project = tr("topbar.noProject", a.locale)
		}
		ui.Text(c, project).Grow(1).MinWidth(0).SingleLine().Ellipsis("…").Tooltip(project)
		if ui.Button(c.Key("desktop"), tr("topbar.showDesktop", a.locale)).Disabled(a.busy || a.state != vm.VMRunning).Clicked() {
			a.showDesktop()
		}
		if ui.Button(c.Key("stop"), tr("topbar.stopVM", a.locale)).Disabled(a.busy || a.state != vm.VMRunning).Clicked() {
			a.stop()
		}
	})
}
