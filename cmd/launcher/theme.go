package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/finch-xu/clawdhome/vm"
)

// toThemeSource 把保存的外观偏好映射到 MyGo 的 ThemeSource。空值和未知值都
// 回到跟随系统。
func toThemeSource(pref string) mygo.ThemeSource {
	switch pref {
	case string(mygo.ThemeLight):
		return mygo.ThemeLight
	case string(mygo.ThemeDark):
		return mygo.ThemeDark
	default:
		return mygo.ThemeSystem
	}
}

func themePreference(pref string) string {
	if source := toThemeSource(pref); source != mygo.ThemeSystem {
		return string(source)
	}
	return ""
}

func themeLabel(pref string) string {
	switch toThemeSource(pref) {
	case mygo.ThemeLight:
		return "settings.light"
	case mygo.ThemeDark:
		return "settings.dark"
	default:
		return "settings.followSystem"
	}
}

func (a *app) cycleTheme(current string) {
	cfg := a.cfg
	switch toThemeSource(current) {
	case mygo.ThemeLight:
		cfg.ThemePref = string(mygo.ThemeDark)
	case mygo.ThemeDark:
		cfg.ThemePref = ""
	default:
		cfg.ThemePref = string(mygo.ThemeLight)
	}
	if err := a.launcher.SaveSettings(cfg); err != nil {
		a.message = errorNotice("error.save-settings", err)
		return
	}
	a.cfg.ThemePref, a.draft.ThemePref = cfg.ThemePref, cfg.ThemePref
	a.appearance.SetTheme(cfg.ThemePref)
	a.message = notice{}
}

func sidebarColor(theme *ui.Theme) ui.Color {
	return theme.Background.Mix(theme.Surface, 0.65)
}

func statusDot(c *ui.Context, color ui.Color) {
	ui.Box(c).Width(6).Height(6).Shrink(0).Radius(3).Background(color)
}

type statusTone uint8

const (
	statusNeutral statusTone = iota
	statusPositive
	statusWarning
	statusNegative
)

type statusItem struct {
	key, label string
	tone       statusTone
}

func (a *app) statusItems(s vm.StatusView) []statusItem {
	vmTone, sshTone, proxyTone, probeTone := statusNeutral, statusNeutral, statusNeutral, statusNeutral
	switch s.VM {
	case vm.VMRunning:
		vmTone = statusPositive
	case vm.VMStarting, vm.VMStopping:
		vmTone = statusWarning
	}
	switch s.SSH {
	case vm.SSHAvailable:
		sshTone = statusPositive
	case vm.SSHUnavailable:
		sshTone = statusNegative
	}
	switch s.Proxy {
	case vm.ProxyConnected:
		proxyTone = statusPositive
	case vm.ProxyFailed:
		proxyTone = statusNegative
	case vm.ProxyConnecting, vm.ProxyWaitingCheck:
		proxyTone = statusWarning
	}
	switch s.Probe {
	case vm.ProbeReachable:
		probeTone = statusPositive
	case vm.ProbeUnreachable:
		probeTone = statusNegative
	case vm.ProbeExpired, vm.ProbeWaiting:
		probeTone = statusWarning
	}
	return []statusItem{
		{"vm", "VM " + tr("vmState."+s.VM, a.locale), vmTone},
		{"ssh", "SSH " + tr("ssh."+s.SSH, a.locale), sshTone},
		{"clash", "Clash " + tr("proxy."+s.Proxy, a.locale), proxyTone},
		{"google", "Google " + tr("probe."+s.Probe, a.locale), probeTone},
	}
}

func statusColor(theme *ui.Theme, tone statusTone) ui.Color {
	switch tone {
	case statusPositive:
		return theme.Success
	case statusWarning:
		return theme.Warning
	case statusNegative:
		return theme.Danger
	default:
		return theme.TextMuted
	}
}
