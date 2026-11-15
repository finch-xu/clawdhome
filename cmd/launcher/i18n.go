package main

import "strings"

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

// t 取当前语言下 key 对应的文案。
func t(key string, loc locale) string { return lookup(messages, key, loc) }

// toLocale 把 MyGo 给出的语言标签（如 "zh-Hans-CN"）归到支持的两种语言之一。
func toLocale(tag string) locale {
	if strings.HasPrefix(tag, "zh") {
		return localeZhHans
	}
	return localeEN
}

var messages = map[locale]map[string]string{
	localeEN: {
		"nav.session":           "Sessions",
		"nav.network":           "Network",
		"nav.settings":          "Settings",
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
		"network.routeTitle":    "Egress Route",
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
		"status.chooseInside":   "Pick a project directory inside the workspace.",
		"theme.system":          "System",
		"theme.light":           "Light",
		"theme.dark":            "Dark",
	},
	localeZhHans: {
		"nav.session":           "会话",
		"nav.network":           "网络",
		"nav.settings":          "设置",
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
		"network.routeTitle":    "出口路由",
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
		"status.chooseInside":   "请选择工作空间内的项目目录。",
		"theme.system":          "系统",
		"theme.light":           "浅色",
		"theme.dark":            "深色",
	},
}
