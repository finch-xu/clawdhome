package main

import (
	"strings"
	"testing"
	"time"

	egress "github.com/finch-xu/clawdhome/lima-egress"
)

func TestStatusDistinguishesWorkingProxyFromBlockedEgress(t *testing.T) {
	now := time.Now()
	snap := egress.GatewayStatus{Running: true, ProxyAddr: "127.0.0.1:1234", Proxy: egress.ProxyConnected,
		Probe: egress.ProbeUnreachable, Reason: "Google HTTP 503", LastProbe: now, UpdatedAt: now}
	status := describeStatus("running", "可用", settings{Port: "1234"}, &snap, now)
	if status.proxy != "已连接" || status.probe != "探测失败" || status.firewall != "外网已阻断" || !strings.Contains(status.probeDetail, "Google HTTP 503") {
		t.Fatalf("status = %+v", status)
	}
	snap.Online, snap.Probe, snap.Until = true, egress.ProbeReachable, now
	status = describeStatus("running", "可用", settings{Port: "1234"}, &snap, now)
	if status.probe != "结果过期" || status.firewall != "外网已阻断" {
		t.Fatalf("expired status = %+v", status)
	}
	status = describeStatus("running", "可用", settings{Port: "2345"}, &snap, now.Add(2*time.Minute))
	if status.proxy != "状态未知" || status.firewall != "状态未知" || !strings.Contains(status.proxyDetail, "下次启动使用 2345") {
		t.Fatalf("saved port and stale snapshot status = %+v", status)
	}
}

func TestSettingsSavedWithoutStartingVM(t *testing.T) {
	l := launcher{home: t.TempDir()}
	want := settings{Port: "59321"}
	if err := l.saveSettings(want); err != nil {
		t.Fatal(err)
	}
	if got := l.settings(); got != want {
		t.Fatalf("settings = %+v; want %+v", got, want)
	}
	if err := l.saveSettings(settings{Port: "65536"}); err == nil || l.settings() != want {
		t.Fatal("invalid port replaced persisted settings")
	}
}
