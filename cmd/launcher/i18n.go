package main

import (
	"fmt"
	"strings"
	"time"

	egress "github.com/finch-xu/clawdhome/lima-egress"
	"github.com/finch-xu/clawdhome/vm"
)

// locale 是应用支持的界面语言。英语是默认语言，也是缺翻译时的回退语言。
type locale string

const (
	localeEN     locale = "en"
	localeZhHans locale = "zh-Hans"
)

// lookup 按 loc 查文案：该语言缺这个 key 时回退到英语，两边都没有时返回 key
// 本身，避免界面上出现空字符串。
func lookup(dicts map[locale]map[string]string, key string, loc locale) string {
	if dict, ok := dicts[loc]; ok {
		if v, ok := dict[key]; ok {
			return v
		}
	}
	if v, ok := dicts[localeEN][key]; ok {
		return v
	}
	return key
}

// tr 取当前语言下 key 对应的文案。名字用 tr 而不是 t，因为 Go 测试里 t 是
// *testing.T 的惯用名，同名会被遮蔽。
func tr(key string, loc locale) string { return lookup(messages, key, loc) }

// toLocale 把 MyGo 给出的语言标签（如 "zh-Hans-CN"）归到支持的两种语言之一。
func toLocale(tag string) locale {
	if strings.HasPrefix(tag, "zh") {
		return localeZhHans
	}
	return localeEN
}

var messages = map[locale]map[string]string{
	localeEN: {
		"nav.sessions":                "Sessions",
		"nav.network":                 "Network",
		"theme.switch":                "Switch appearance",
		"session.listTitle":           "Sessions (%d)",
		"session.new":                 "New Session",
		"session.selectToAttach":      "Select a session in the list to connect",
		"network.routeTitle":          "Egress Route",
		"network.statusTitle":         "Connection Status",
		"network.routeClash":          "Use the host’s Clash proxy",
		"network.routeSystem":         "Use the host’s system route",
		"network.routeBlocked":        "Keep only local SSH access to the VM",
		"settings.projectTitle":       "Project",
		"settings.proxyTitle":         "Network Proxy",
		"settings.appearanceTitle":    "Appearance and Language",
		"settings.vmEnvTitle":         "VM Environment",
		"settings.guestLocale":        "Language & Region",
		"settings.guestZone":          "Time Zone",
		"settings.portRestart":        "Clash SOCKS5 / Mixed port changes take effect on the next VM start.",
		"session.exitSignal":          "Ended by signal %s",
		"session.exitUnknown":         "Exited · result unknown",
		"network.route":               "Current route",
		"network.handshake":           "Clash handshake",
		"network.probe":               "Google probe",
		"network.firewall":            "Firewall policy",
		"network.dohPolicy":           "Google, with Cloudflare as fallback",
		"network.dohBlocked":          "Unavailable while internet access is blocked",
		"error.port-required":         "Set a valid Clash port before starting the VM",
		"error.project":               "Choose a folder inside the workspace",
		"error.share":                 "Could not prepare the shared folders",
		"error.save-settings":         "Could not save settings",
		"error.prepare":               "Could not prepare or start the VM",
		"error.mounts-changed":        "Stop the VM before updating shared folders",
		"error.desktop":               "Could not open the VM desktop",
		"error.desktop-restart":       "Restart the VM to enable the desktop button",
		"error.command":               "Enter a CLI command",
		"error.create-session":        "Could not start the session",
		"error.stop":                  "Could not stop the VM",
		"error.kill-session":          "Could not end the session",
		"error.route":                 "Could not change the preferred route",
		"error.snapshot":              "Could not read the VM network snapshot",
		"error.attach":                "Could not attach to the session. Select it to retry.",
		"error.choose-directory":      "Could not open the folder picker",
		"error.operation":             "The operation failed",
		"probeReason.gateway-stopped": "The gateway stopped",
		"probeReason.expired":         "The Google probe expired",
		"probeReason.waiting":         "Waiting for the Google probe",
		"probeReason.blocked":         "Internet access was manually blocked",
		"probeReason.proxy-failed":    "Clash SOCKS5 is unavailable",
		"probeReason.request-invalid": "The Google probe request was invalid",
		"probeReason.https-failed":    "The Google HTTPS probe failed on this route",
		"probeReason.http-status":     "Google HTTP %d",
		"vmState.running":             "Running",
		"vmState.stopped":             "Stopped",
		"vmState.starting":            "Starting",
		"vmState.stopping":            "Stopping",
		"vmState.unknown":             "Unknown",
		"proxy.waiting-check":         "Checking",
		"proxy.unconfigured":          "Not configured",
		"proxy.connecting":            "Connecting",
		"proxy.connected":             "Connected",
		"proxy.failed":                "Failed",
		"proxy.disabled":              "Disabled",
		"proxy.unknown":               "Unknown",
		"probe.waiting":               "Waiting",
		"probe.reachable":             "Reachable",
		"probe.unreachable":           "Unreachable",
		"probe.expired":               "Expired",
		"probe.skipped":               "Not probed",
		"firewall.unknown":            "Unknown",
		"firewall.not-running":        "Not running",
		"firewall.blocked":            "Internet blocked",
		"firewall.proxy-only":         "Proxy egress only",
		"firewall.system-route":       "System route egress",
		"status.localSSH":             "Local SSH ",
		"proxyDetail.saved-port":      "Saved %s",
		"proxyDetail.current-addr":    "Current %s",
		"proxyDetail.port-cleared":    "Current %s · cleared in settings, applies next launch",
		"proxyDetail.port-pending":    "Current %s · next launch uses %s",
		"proxyDetail.egress-route":    "Egress %s",
		"firewallDetail.no-gateway":   "No gateway status from this VM yet",
		"firewallDetail.stale":        "The gateway has not reported for a while",
		"firewallDetail.blocked":      "New public connections are blocked; existing ones close on shutdown",
		"firewallDetail.proxy-only":   "Only IPv4 TCP and DoH egress through the proxy",
		"firewallDetail.system-route": "Only IPv4 TCP and DoH egress through the host's system route",
		"firewallDetail.lease-expiry": "The probe lease expired; the gateway closed public connections",
		"probeDetail.lastProbe":       "last %s",
		"app.title":                   "macOS CLI Launcher",
		"menu.sessions":               "Sessions",
		"menu.startClaude":            "Start Claude Code",
		"menu.openTerminal":           "Open Terminal",
		"ssh.checking":                "Checking",
		"ssh.available":               "Available",
		"ssh.unavailable":             "Unavailable for now",
		"ssh.not-running":             "Not running",
		"ssh.unknown":                 "Unknown",
		"vm.stopping":                 "Stopping VM…",
		"dialog.chooseProject":        "Pick a project directory inside the workspace.",
		"topbar.showDesktop":          "Show VM Desktop",
		"topbar.stopVM":               "Stop VM",
		"topbar.noProject":            "No project selected",
		"session.commandLabel":        "CLI command",
		"session.commandHint":         "CLI command",
		"session.run":                 "Run",
		"session.claudeCode":          "Claude Code",
		"session.terminal":            "Terminal",
		"session.end":                 "End Session",
		"session.empty":               "Run a CLI, or pick an existing session.",
		"session.starting":            "Starting session…",
		"session.reattach":            "The terminal connection ended. Pick a session to reconnect.",
		"session.uptimeUnknown":       "uptime unknown",
		"session.exitCode":            "Exited · code %d",
		"network.snapshotTitle":       "VM Network Snapshot",
		"route.system":                "System Route",
		"route.blocked":               "Offline",
		"route.unknown":               "Unknown",
		"network.refresh":             "Refresh",
		"network.testAnthropic":       "Test Anthropic",
		"network.reading":             "Reading VM network state…",
		"network.snapshotAt":          "VM network snapshot · %s",
		"network.nextLaunch":          "Next launch uses ",
		"network.unknownEgress":       "Current egress unknown · restart the VM if it runs an older gateway",
		"network.switchingTo":         " · switching to ",
		"settings.title":              "Settings",
		"settings.chooseDir":          "Choose Directory",
		"settings.notSelected":        "Not selected",
		"settings.projectHint":        "New sessions use this folder. Switching projects does not require restarting the VM.",
		"settings.clashPort":          "Clash port",
		"settings.portHint":           "Port",
		"settings.testPort":           "Test Port",
		"settings.cancel":             "Cancel",
		"settings.save":               "Save Settings",
		"settings.saved":              "Settings saved",
		"settings.portRange":          "Enter a port between 1 and 65535",
		"settings.portTesting":        "Testing port %s…",
		"settings.portOK":             "Port %s: SOCKS5 handshake succeeded",
		"settings.portFailed":         "Port %s: SOCKS5 handshake failed",
		"settings.probePolicy":        "Probe policy  every %ds · %ds timeout · recovers after %d successes · backs off on 429",
		"settings.appearance":         "Appearance",
		"settings.language":           "App language",
		"settings.followSystem":       "System",
		"settings.light":              "Light",
		"settings.dark":               "Dark",
		"settings.langHint":           "Following the system picks the language automatically; English is the default.",
		"status.details":              "Status Details",
		"status.hideDetails":          "Hide Details",
		"status.firewall":             "Firewall",
	},
	localeZhHans: {
		"nav.sessions":                "会话",
		"nav.network":                 "网络",
		"theme.switch":                "切换外观",
		"session.listTitle":           "会话（%d）",
		"session.new":                 "新建会话",
		"session.selectToAttach":      "选择列表中的会话以连接",
		"network.routeTitle":          "出口路由",
		"network.statusTitle":         "连接状态",
		"network.routeClash":          "经本机 Clash 代理出站",
		"network.routeSystem":         "沿用宿主机网络出口",
		"network.routeBlocked":        "仅保留宿主机到 VM 的本地 SSH",
		"settings.projectTitle":       "项目",
		"settings.proxyTitle":         "网络代理",
		"settings.appearanceTitle":    "外观与语言",
		"settings.vmEnvTitle":         "VM 环境",
		"settings.guestLocale":        "语言与地区",
		"settings.guestZone":          "时区",
		"settings.portRestart":        "Clash SOCKS5 / Mixed 端口改动在下次启动 VM 时生效。",
		"session.exitSignal":          "已结束 · 信号 %s",
		"session.exitUnknown":         "已结束 · 退出结果未知",
		"network.route":               "当前出口",
		"network.handshake":           "Clash 握手",
		"network.probe":               "Google 探测",
		"network.firewall":            "防火墙策略",
		"network.dohPolicy":           "Google，失败时回退 Cloudflare",
		"network.dohBlocked":          "断开公网时不可用",
		"error.port-required":         "请先在设置中填写有效的 Clash 端口",
		"error.project":               "请在工作空间内选择项目目录",
		"error.share":                 "无法准备共享目录",
		"error.save-settings":         "无法保存设置",
		"error.prepare":               "无法准备或启动 VM",
		"error.mounts-changed":        "请先停止 VM，再更新共享目录",
		"error.desktop":               "无法打开 VM 桌面",
		"error.desktop-restart":       "请重启 VM 以启用桌面窗口按钮",
		"error.command":               "请输入 CLI 命令",
		"error.create-session":        "无法启动会话",
		"error.stop":                  "无法停止 VM",
		"error.kill-session":          "无法结束会话",
		"error.route":                 "无法更改首选出口",
		"error.snapshot":              "无法读取 VM 网络快照",
		"error.attach":                "无法连接会话，选择该会话可重试",
		"error.choose-directory":      "无法打开目录选择窗口",
		"error.operation":             "操作失败",
		"probeReason.gateway-stopped": "网关已停止",
		"probeReason.expired":         "Google 探测已过期",
		"probeReason.waiting":         "等待 Google 探测",
		"probeReason.blocked":         "已手动断开公网",
		"probeReason.proxy-failed":    "Clash SOCKS5 不可用",
		"probeReason.request-invalid": "Google 探测请求无效",
		"probeReason.https-failed":    "当前出口的 Google HTTPS 探测失败",
		"probeReason.http-status":     "Google HTTP %d",
		"vmState.running":             "运行中",
		"vmState.stopped":             "已停止",
		"vmState.starting":            "启动中",
		"vmState.stopping":            "停止中",
		"vmState.unknown":             "状态未知",
		"proxy.waiting-check":         "等待检测",
		"proxy.unconfigured":          "未配置",
		"proxy.connecting":            "连接中",
		"proxy.connected":             "已连接",
		"proxy.failed":                "连接失败",
		"proxy.disabled":              "已停用",
		"proxy.unknown":               "状态未知",
		"probe.waiting":               "等待探测",
		"probe.reachable":             "可达",
		"probe.unreachable":           "探测失败",
		"probe.expired":               "结果过期",
		"probe.skipped":               "未探测",
		"firewall.unknown":            "状态未知",
		"firewall.not-running":        "未运行",
		"firewall.blocked":            "外网已阻断",
		"firewall.proxy-only":         "仅允许经代理出站",
		"firewall.system-route":       "系统路由出站",
		"status.localSSH":             "本地 SSH ",
		"proxyDetail.saved-port":      "已保存 %s",
		"proxyDetail.current-addr":    "当前 %s",
		"proxyDetail.port-cleared":    "当前 %s · 已清空设置，下次启动生效",
		"proxyDetail.port-pending":    "当前 %s · 下次启动使用 %s",
		"proxyDetail.egress-route":    "出口 %s",
		"firewallDetail.no-gateway":   "尚未收到当前 VM 的网关状态",
		"firewallDetail.stale":        "网关状态长时间未更新",
		"firewallDetail.blocked":      "公网新连接已阻断，已有公网连接在关断时关闭",
		"firewallDetail.proxy-only":   "仅 IPv4 TCP 和 DoH 经代理出站",
		"firewallDetail.system-route": "仅 IPv4 TCP 和 DoH 经宿主机系统路由出站",
		"firewallDetail.lease-expiry": "探测租期已过，网关按策略关闭公网连接",
		"probeDetail.lastProbe":       "上次 %s",
		"app.title":                   "macOS CLI 启动器",
		"menu.sessions":               "会话",
		"menu.startClaude":            "启动 Claude Code",
		"menu.openTerminal":           "打开终端",
		"ssh.checking":                "等待检查",
		"ssh.available":               "可用",
		"ssh.unavailable":             "暂不可用",
		"ssh.not-running":             "未运行",
		"ssh.unknown":                 "状态未知",
		"vm.stopping":                 "正在停止 VM…",
		"dialog.chooseProject":        "请选择工作空间内的项目目录。",
		"topbar.showDesktop":          "显示 VM 桌面",
		"topbar.stopVM":               "停止 VM",
		"topbar.noProject":            "未选择项目",
		"session.commandLabel":        "CLI 命令",
		"session.commandHint":         "CLI 命令",
		"session.run":                 "运行",
		"session.claudeCode":          "Claude Code",
		"session.terminal":            "终端",
		"session.end":                 "结束会话",
		"session.empty":               "运行一个 CLI，或选择已有会话。",
		"session.starting":            "正在启动会话…",
		"session.reattach":            "终端连接已结束，可以重新选择会话。",
		"session.uptimeUnknown":       "运行时长未知",
		"session.exitCode":            "已结束 · 退出码 %d",
		"network.snapshotTitle":       "VM 网络快照",
		"route.system":                "系统路由",
		"route.blocked":               "断开公网",
		"route.unknown":               "未知",
		"network.refresh":             "刷新连接",
		"network.testAnthropic":       "测试 Anthropic",
		"network.reading":             "正在读取 VM 网络信息…",
		"network.snapshotAt":          "VM 网络快照 · %s",
		"network.nextLaunch":          "下次启动使用 ",
		"network.unknownEgress":       "当前出口未知 · 若 VM 正在运行旧版网关，请重启 VM",
		"network.switchingTo":         " · 正在切换到 ",
		"settings.title":              "设置",
		"settings.chooseDir":          "选择目录",
		"settings.notSelected":        "未选择",
		"settings.projectHint":        "新会话使用选定项目，切换项目无需重启 VM。",
		"settings.clashPort":          "Clash 端口",
		"settings.portHint":           "端口",
		"settings.testPort":           "测试端口",
		"settings.cancel":             "取消",
		"settings.save":               "保存设置",
		"settings.saved":              "设置已保存",
		"settings.portRange":          "请输入 1–65535 之间的端口",
		"settings.portTesting":        "正在测试端口 %s…",
		"settings.portOK":             "端口 %s 的 SOCKS5 握手成功",
		"settings.portFailed":         "端口 %s 的 SOCKS5 握手失败",
		"settings.probePolicy":        "探测策略  通常每 %d 秒 · %d 秒超时 · 连续 %d 次成功后恢复 · 429 时退避",
		"settings.appearance":         "外观",
		"settings.language":           "应用语言",
		"settings.followSystem":       "跟随系统",
		"settings.light":              "浅色",
		"settings.dark":               "深色",
		"settings.langHint":           "跟随系统时按系统语言自动选择，默认使用英语。",
		"status.details":              "状态详情",
		"status.hideDetails":          "收起详情",
		"status.firewall":             "防火墙",
	},
}

// formatUptime 把运行时长写成人类可读的形式，按当前语言选单位。不足一分钟时
// 报秒数，否则刚启动的会话会显示「0 分钟」，看起来像没跑起来；超过一天按天报，
// 因为「3 天」比「72 小时」好读。
func formatUptime(d time.Duration, loc locale) string {
	days := int(d.Hours()) / 24
	hours, minutes, seconds := int(d.Hours())%24, int(d.Minutes())%60, int(d.Seconds())%60
	zh := loc == localeZhHans
	switch {
	case days > 0 && zh:
		return fmt.Sprintf("运行 %d 天 %d 小时", days, hours)
	case days > 0:
		return fmt.Sprintf("running %dd %dh", days, hours)
	case int(d.Hours()) > 0 && zh:
		return fmt.Sprintf("运行 %d 小时 %d 分", hours, minutes)
	case int(d.Hours()) > 0:
		return fmt.Sprintf("running %dh %dm", hours, minutes)
	case minutes > 0 && zh:
		return fmt.Sprintf("运行 %d 分钟", minutes)
	case minutes > 0:
		return fmt.Sprintf("running %dm", minutes)
	case zh:
		return fmt.Sprintf("运行 %d 秒", seconds)
	default:
		return fmt.Sprintf("running %ds", seconds)
	}
}

// localePrefTag 把保存的语言偏好收敛到支持的两种语言之一，其余（含手改坏的
// 配置）一律当作跟随系统。必须有这道校验：mygo.App.SetLocale 对无法识别的标签
// 会 panic，而空字符串是它认可的「清除覆盖」。
func localePrefTag(pref string) string {
	switch pref {
	case string(localeEN), string(localeZhHans):
		return pref
	default:
		return ""
	}
}

// proxyDetail 把代理详情的状态码和数据拼成当前语言的一句话。
func (a *app) proxyDetail(s vm.StatusView) string {
	var text string
	switch s.ProxyDetail {
	case vm.ProxyDetailNone:
	case vm.ProxyDetailSavedPort:
		text = fmt.Sprintf(tr("proxyDetail.saved-port", a.locale), s.SavedProxyAddr)
	case vm.ProxyDetailCurrentAddr:
		text = fmt.Sprintf(tr("proxyDetail.current-addr", a.locale), s.CurrentProxyAddr)
	case vm.ProxyDetailPortCleared:
		text = fmt.Sprintf(tr("proxyDetail.port-cleared", a.locale), s.CurrentProxyAddr)
	case vm.ProxyDetailPortPending:
		text = fmt.Sprintf(tr("proxyDetail.port-pending", a.locale), s.CurrentProxyAddr, s.PendingPort)
	case vm.ProxyDetailEgressRoute:
		text = fmt.Sprintf(tr("proxyDetail.egress-route", a.locale), a.routeLabel(egress.Route(s.EgressRoute)))
	}

	return text
}

// probeDetail 把探测原因和最近一次探测时间拼成一句话。
func (a *app) probeDetail(s vm.StatusView) string {
	text := ""
	if s.ProbeReasonCode != "" {
		key := "probeReason." + s.ProbeReasonCode
		if translated := tr(key, a.locale); translated != key {
			text = translated
			if s.ProbeReasonCode == egress.ReasonHTTPStatus {
				text = fmt.Sprintf(text, s.ProbeHTTPStatus)
			}
		}
	}
	if text == "" && s.ProbeReason != "" {
		text = tr("probe."+s.Probe, a.locale)
	}
	if s.LastProbe.IsZero() {
		return text
	}
	last := fmt.Sprintf(tr("probeDetail.lastProbe", a.locale), s.LastProbe.Local().Format("01-02 15:04:05"))
	if text == "" {
		return last
	}
	return text + "  ·  " + last
}

// firewallDetail 查出防火墙详情对应的文案。
func (a *app) firewallDetail(s vm.StatusView) string {
	if s.FirewallDetail == vm.FirewallDetailNone {
		return ""
	}
	return tr("firewallDetail."+s.FirewallDetail, a.locale)
}
