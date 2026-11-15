package main

import (
	"fmt"
	"strings"
	"time"

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
		"vm.running":            "Running",
		"vm.stopped":            "Stopped",
		"vm.starting":           "Starting",
		"vm.stopping.state":     "Stopping",
		"vm.unknown":            "Unknown",
		"proxy.waiting-check":   "Checking",
		"proxy.unconfigured":    "Not configured",
		"proxy.connecting":      "Connecting",
		"proxy.connected":       "Connected",
		"proxy.failed":          "Failed",
		"proxy.disabled":        "Disabled",
		"proxy.unknown":         "Unknown",
		"probe.waiting":         "Waiting",
		"probe.reachable":       "Reachable",
		"probe.unreachable":     "Unreachable",
		"probe.expired":         "Expired",
		"probe.skipped":         "Not probed",
		"firewall.unknown":      "Unknown",
		"firewall.not-running":  "Not running",
		"firewall.blocked":      "Internet blocked",
		"firewall.proxy-only":   "Proxy egress only",
		"firewall.system-route": "System route egress",
		"status.localSSH":       "Local SSH ",
		"app.title":             "macOS CLI Launcher",
		"menu.sessions":         "Sessions",
		"menu.startClaude":      "Start Claude Code",
		"menu.openTerminal":     "Open Terminal",
		"ssh.checking":          "Checking",
		"ssh.available":         "Available",
		"ssh.unavailable":       "Unavailable for now",
		"ssh.not-running":       "Not running",
		"ssh.unknown":           "Unknown",
		"vm.stopping":           "Stopping VM…",
		"dialog.chooseProject":  "Pick a project directory inside the workspace.",
		"network.readFailed":    "Read failed (%v)",
		"topbar.showDesktop":    "Show VM Desktop",
		"topbar.stopVM":         "Stop VM",
		"topbar.noProject":      "No project selected",
		"session.commandLabel":  "CLI command",
		"session.commandHint":   "CLI command",
		"session.run":           "Run",
		"session.claudeCode":    "Claude Code",
		"session.terminal":      "Terminal",
		"session.end":           "End Session",
		"session.empty":         "Run a CLI, or pick an existing session.",
		"session.starting":      "Starting session…",
		"session.reattach":      "The terminal connection ended. Pick a session to reconnect.",
		"session.exited":        "Exited",
		"session.uptimeUnknown": "uptime unknown",
		"network.snapshotTitle": "VM Network Snapshot",
		"route.system":          "System Route",
		"route.blocked":         "Offline",
		"route.unknown":         "Unknown",
		"network.refresh":       "Refresh",
		"network.testAnthropic": "Test Anthropic",
		"network.backToTerm":    "Back to Terminal",
		"network.reading":       "Reading VM network state…",
		"network.snapshotAt":    "VM network snapshot · ",
		"network.currentEgress": "Current egress ",
		"network.nextLaunch":    "Next launch uses ",
		"network.unknownEgress": "Current egress unknown · restart the VM if it runs an older gateway",
		"network.switchingTo":   " · switching to ",
		"settings.title":        "Settings",
		"settings.projectDir":   "Project directory  ",
		"settings.chooseDir":    "Choose Directory",
		"settings.notSelected":  "Not selected",
		"settings.projectHint":  "New sessions use the selected project. Port changes take effect on the next VM start.",
		"settings.clashPort":    "Clash port",
		"settings.portHint":     "Port",
		"settings.testPort":     "Test Port",
		"settings.cancel":       "Cancel",
		"settings.save":         "Save Settings",
		"settings.saved":        "Settings saved",
		"settings.portRange":    "Enter a port between 1 and 65535",
		"settings.portTesting":  "Testing port ",
		"settings.portOK":       " SOCKS5 handshake succeeded",
		"settings.portFailed":   " SOCKS5 handshake failed · ",
		"settings.vmEnv":        "VM environment  US English · US region · ",
		"settings.probePolicy":  "Probe policy  every %ds · %ds timeout · recovers after %d successes · backs off on 429",
		"settings.appearance":   "Appearance",
		"settings.language":     "App language",
		"settings.followSystem": "System",
		"settings.light":        "Light",
		"settings.dark":         "Dark",
		"settings.langHint":     "Following the system picks the language automatically; English is the default.",
		"status.details":        "Status Details",
		"status.hideDetails":    "Hide Details",
		"status.network":        "Network",
		"status.firewall":       "Firewall",
	},
	localeZhHans: {
		"vm.running":            "运行中",
		"vm.stopped":            "已停止",
		"vm.starting":           "启动中",
		"vm.stopping.state":     "停止中",
		"vm.unknown":            "状态未知",
		"proxy.waiting-check":   "等待检测",
		"proxy.unconfigured":    "未配置",
		"proxy.connecting":      "连接中",
		"proxy.connected":       "已连接",
		"proxy.failed":          "连接失败",
		"proxy.disabled":        "已停用",
		"proxy.unknown":         "状态未知",
		"probe.waiting":         "等待探测",
		"probe.reachable":       "可达",
		"probe.unreachable":     "探测失败",
		"probe.expired":         "结果过期",
		"probe.skipped":         "未探测",
		"firewall.unknown":      "状态未知",
		"firewall.not-running":  "未运行",
		"firewall.blocked":      "外网已阻断",
		"firewall.proxy-only":   "仅允许经代理出站",
		"firewall.system-route": "系统路由出站",
		"status.localSSH":       "本地 SSH ",
		"app.title":             "macOS CLI 启动器",
		"menu.sessions":         "会话",
		"menu.startClaude":      "启动 Claude Code",
		"menu.openTerminal":     "打开终端",
		"ssh.checking":          "等待检查",
		"ssh.available":         "可用",
		"ssh.unavailable":       "暂不可用",
		"ssh.not-running":       "未运行",
		"ssh.unknown":           "状态未知",
		"vm.stopping":           "正在停止 VM…",
		"dialog.chooseProject":  "请选择工作空间内的项目目录。",
		"network.readFailed":    "读取失败 (%v)",
		"topbar.showDesktop":    "显示 VM 桌面",
		"topbar.stopVM":         "停止 VM",
		"topbar.noProject":      "未选择项目",
		"session.commandLabel":  "CLI 命令",
		"session.commandHint":   "CLI 命令",
		"session.run":           "运行",
		"session.claudeCode":    "Claude Code",
		"session.terminal":      "终端",
		"session.end":           "结束会话",
		"session.empty":         "运行一个 CLI，或选择已有会话。",
		"session.starting":      "正在启动会话…",
		"session.reattach":      "终端连接已结束，可以重新选择会话。",
		"session.exited":        "已退出",
		"session.uptimeUnknown": "运行时长未知",
		"network.snapshotTitle": "VM 网络快照",
		"route.system":          "系统路由",
		"route.blocked":         "断开公网",
		"route.unknown":         "未知",
		"network.refresh":       "刷新连接",
		"network.testAnthropic": "测试 Anthropic",
		"network.backToTerm":    "返回终端",
		"network.reading":       "正在读取 VM 网络信息…",
		"network.snapshotAt":    "VM 网络快照 · ",
		"network.currentEgress": "当前出口 ",
		"network.nextLaunch":    "下次启动使用 ",
		"network.unknownEgress": "当前出口未知 · 若 VM 正在运行旧版网关，请重启 VM",
		"network.switchingTo":   " · 正在切换到 ",
		"settings.title":        "设置",
		"settings.projectDir":   "项目目录  ",
		"settings.chooseDir":    "选择目录",
		"settings.notSelected":  "未选择",
		"settings.projectHint":  "新会话使用选定项目。端口改动在下次启动 VM 时生效。",
		"settings.clashPort":    "Clash 端口",
		"settings.portHint":     "端口",
		"settings.testPort":     "测试端口",
		"settings.cancel":       "取消",
		"settings.save":         "保存设置",
		"settings.saved":        "设置已保存",
		"settings.portRange":    "请输入 1–65535 之间的端口",
		"settings.portTesting":  "正在测试端口 ",
		"settings.portOK":       " SOCKS5 握手成功",
		"settings.portFailed":   " SOCKS5 握手失败 · ",
		"settings.vmEnv":        "VM 环境  美国英语 · 美国地区 · ",
		"settings.probePolicy":  "探测策略  通常每 %d 秒 · %d 秒超时 · 连续 %d 次成功后恢复 · 429 时退避",
		"settings.appearance":   "外观",
		"settings.language":     "应用语言",
		"settings.followSystem": "跟随系统",
		"settings.light":        "浅色",
		"settings.dark":         "深色",
		"settings.langHint":     "跟随系统时按系统语言自动选择，默认使用英语。",
		"status.details":        "状态详情",
		"status.hideDetails":    "收起详情",
		"status.network":        "网络连接",
		"status.firewall":       "防火墙",
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

// statusSummary 是状态条常驻那一行的文案：四项状态各查字典翻译。
func (a *app) statusSummary(s vm.StatusView) string {
	return fmt.Sprintf("VM %s  ·  Clash %s  ·  Google %s  ·  %s %s",
		tr("vm."+s.VM, a.locale), tr("proxy."+s.Proxy, a.locale),
		tr("probe."+s.Probe, a.locale),
		tr("status.firewall", a.locale), tr("firewall."+s.Firewall, a.locale))
}
