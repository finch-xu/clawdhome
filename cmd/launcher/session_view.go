package main

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"github.com/finch-xu/clawdhome/vm"
)

func (a *app) sessionsView(c *ui.Context) {
	ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c.Key("session-list")).Width(180).Shrink(0).MinHeight(0).Padding(8).Gap(6).Children(func() {
			ui.Text(c, fmt.Sprintf(tr("session.listTitle", a.locale), len(a.sessions))).Bold()
			ui.Scroll(c.Key("list")).Grow(1).MinHeight(0).Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					now := time.Now()
					for _, s := range a.sessions {
						a.sessionRow(c.Key(s.ID), s, now)
					}
				})
			})
			if ui.Button(c.Key("new-session"), tr("session.new", a.locale)).Disabled(a.busy).Clicked() {
				a.newSessionOpen = !a.newSessionOpen
			}
		})
		ui.Divider(c)
		ui.Column(c.Key("session-content")).Grow(1).MinWidth(0).MinHeight(0).Children(func() {
			if a.newSessionOpen {
				a.sessionComposer(c.Key("composer"))
				ui.Divider(c)
			}
			if a.selected != "" {
				a.terminalToolbar(c.Key("terminal-toolbar"))
				ui.Divider(c)
			}
			if term := a.terms[a.selected]; term != nil {
				ui.Box(c.Key("terminal-shell")).Grow(1).MinHeight(0).MinWidth(0).Children(func() {
					view := terminal.View(c.Key(a.selected), term).Fill()
					if !a.newSessionOpen {
						view.AutoFocus()
					}
				})
			} else {
				key := "session.empty"
				if a.selected != "" {
					key = "session.selectToAttach"
				}
				ui.Box(c.Key("empty")).Grow(1).MinHeight(0).Padding(10).Center().Children(func() {
					ui.Text(c, tr(key, a.locale)).TextColor(c.Theme().TextMuted)
				})
			}
		})
	})
}

func (a *app) sessionRow(c *ui.Context, s vm.Session, now time.Time) {
	name := a.sessionName(s)
	button := ui.Button(c, "").FillWidth().Padding(6, 8).Label(name).Disabled(a.busy).
		Selected(a.selected == s.ID).Tooltip(name).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Row(c).Gap(6).Children(func() {
				color := c.Theme().Success
				if s.Exited {
					color = c.Theme().TextMuted
					if s.ExitSignal != "" || (s.ExitCodeKnown && s.ExitCode != 0) {
						color = c.Theme().Danger
					}
				}
				statusDot(c, color)
				ui.Text(c, name).Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
			})
			ui.Text(c, a.sessionMeta(s, now)).FontSize(11).TextColor(c.Theme().TextMuted)
		})
	})
	if button.Clicked() {
		a.attach(s.ID)
	}
}

func (a *app) sessionComposer(c *ui.Context) {
	ui.Column(c).Gap(6).Padding(8, 12).Children(func() {
		ui.Row(c.Key("command")).Gap(6).Children(func() {
			ui.TextInput(c, &a.command).Grow(1).MinWidth(0).AutoFocus().
				Label(tr("session.commandLabel", a.locale)).Placeholder(tr("session.commandHint", a.locale)).Disabled(a.busy)
			if ui.PrimaryButton(c, tr("session.run", a.locale)).Disabled(a.busy).Clicked() {
				a.run(a.command)
			}
		})
		ui.Row(c.Key("shortcuts")).Gap(6).Children(func() {
			if ui.Button(c, tr("session.claudeCode", a.locale)).Disabled(a.busy).Clicked() {
				a.run(claudeCommand)
			}
			if ui.Button(c, tr("session.terminal", a.locale)).Disabled(a.busy).Clicked() {
				a.run(terminalCommand)
			}
		})
	})
}

func (a *app) terminalToolbar(c *ui.Context) {
	name := a.selected
	for _, s := range a.sessions {
		if s.ID == a.selected {
			name = a.sessionName(s)
			break
		}
	}
	ui.Row(c).Gap(8).Padding(6, 10).Children(func() {
		ui.Text(c, name).Bold().Grow(1).MinWidth(0).SingleLine().Ellipsis("…").Tooltip(name)
		if ui.Button(c.Key("end"), tr("session.end", a.locale)).Disabled(a.busy).Clicked() {
			a.kill()
		}
	})
}
