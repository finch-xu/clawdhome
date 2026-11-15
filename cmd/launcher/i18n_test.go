package main

import (
	"strings"
	"testing"
	"time"
)

func TestLookupFallsBackToEnglishWhenKeyMissingInLocale(t *testing.T) {
	dicts := map[locale]map[string]string{
		localeEN:     {"greeting": "Hello"},
		localeZhHans: {}, // 这个语言缺该 key
	}
	if got := lookup(dicts, "greeting", localeZhHans); got != "Hello" {
		t.Fatalf("lookup() = %q, want English fallback %q", got, "Hello")
	}
}

func TestLookupReturnsKeyWhenMissingEverywhere(t *testing.T) {
	dicts := map[locale]map[string]string{localeEN: {}, localeZhHans: {}}
	if got := lookup(dicts, "totally.missing.key", localeEN); got != "totally.missing.key" {
		t.Fatalf("lookup() = %q, want the key itself as last-resort fallback", got)
	}
}

func TestLookupPrefersRequestedLocale(t *testing.T) {
	dicts := map[locale]map[string]string{
		localeEN:     {"greeting": "Hello"},
		localeZhHans: {"greeting": "你好"},
	}
	if got := lookup(dicts, "greeting", localeZhHans); got != "你好" {
		t.Fatalf("lookup() = %q, want %q", got, "你好")
	}
}

func TestToLocaleMapsChineseTagsToSimplified(t *testing.T) {
	for _, tag := range []string{"zh-Hans", "zh-Hans-CN", "zh"} {
		if got := toLocale(tag); got != localeZhHans {
			t.Fatalf("toLocale(%q) = %q, want %q", tag, got, localeZhHans)
		}
	}
	for _, tag := range []string{"en", "en-US", "fr", ""} {
		if got := toLocale(tag); got != localeEN {
			t.Fatalf("toLocale(%q) = %q, want %q (English is the default)", tag, got, localeEN)
		}
	}
}

func TestEveryEnglishKeyHasChineseTranslation(t *testing.T) {
	for key := range messages[localeEN] {
		if _, ok := messages[localeZhHans][key]; !ok {
			t.Errorf("key %q has no zh-Hans translation", key)
		}
	}
	for key := range messages[localeZhHans] {
		if _, ok := messages[localeEN][key]; !ok {
			t.Errorf("key %q exists only in zh-Hans; English is the fallback and must have it", key)
		}
	}
}

func TestFormatUptimeShowsSecondsUnderAMinute(t *testing.T) {
	// 刚启动的会话不能显示"0 分钟"，那看起来像没跑起来。
	for _, d := range []time.Duration{0, 30 * time.Second, 59 * time.Second} {
		for _, loc := range []locale{localeEN, localeZhHans} {
			got := formatUptime(d, loc)
			if strings.Contains(got, "0 分钟") || strings.Contains(got, "0m") {
				t.Fatalf("formatUptime(%v, %q) = %q, want seconds instead of a zero minute count", d, loc, got)
			}
		}
	}
}

func TestFormatUptimeUsesDaysBeyondADay(t *testing.T) {
	got := formatUptime(72*time.Hour+30*time.Minute, localeEN)
	if !strings.Contains(got, "3d") {
		t.Fatalf("formatUptime(72h30m) = %q, want it counted in days", got)
	}
	gotZh := formatUptime(72*time.Hour+30*time.Minute, localeZhHans)
	if !strings.Contains(gotZh, "3 天") {
		t.Fatalf("formatUptime(72h30m, zh) = %q, want it counted in days", gotZh)
	}
}

// 这些 key 通过变量传给 tr()，静态搜索找不到它们的使用处，容易在清理字典时被
// 当成死 key 删掉；删掉后界面会显示 key 名本身。这个测试把它们钉住。
func TestDynamicallyLookedUpKeysExist(t *testing.T) {
	keys := []string{
		// appearanceRow 的三个外观选项
		"settings.followSystem", "settings.light", "settings.dark",
		// statusSummary 和状态详情按状态码拼出来的 key
		"vm.running", "vm.stopped", "vm.starting", "vm.stopping.state", "vm.unknown",
		"proxy.waiting-check", "proxy.unconfigured", "proxy.connecting",
		"proxy.connected", "proxy.failed", "proxy.disabled", "proxy.unknown",
		"probe.waiting", "probe.reachable", "probe.unreachable", "probe.expired", "probe.skipped",
		"firewall.unknown", "firewall.not-running", "firewall.blocked",
		"firewall.proxy-only", "firewall.system-route",
		"ssh.checking", "ssh.available", "ssh.unavailable", "ssh.not-running", "ssh.unknown",
		// routeLabel 的路由名
		"route.system", "route.blocked", "route.unknown",
	}
	for _, key := range keys {
		for _, loc := range []locale{localeEN, localeZhHans} {
			if got := tr(key, loc); got == key {
				t.Errorf("key %q missing from the %q dictionary (tr returned the key itself)", key, loc)
			}
		}
	}
}
