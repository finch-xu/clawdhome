package vm

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
	status := DescribeStatus("running", "可用", Settings{Port: "1234"}, &snap, now)
	if status.Proxy != "已连接" || status.Probe != "探测失败" || status.Firewall != "外网已阻断" || !strings.Contains(status.ProbeDetail, "Google HTTP 503") {
		t.Fatalf("status = %+v", status)
	}
	snap.Online, snap.Probe, snap.Until = true, egress.ProbeReachable, now
	status = DescribeStatus("running", "可用", Settings{Port: "1234"}, &snap, now)
	if status.Probe != "结果过期" || status.Firewall != "外网已阻断" {
		t.Fatalf("expired status = %+v", status)
	}
	status = DescribeStatus("running", "可用", Settings{Port: "2345"}, &snap, now.Add(2*time.Minute))
	if status.Proxy != "状态未知" || status.Firewall != "状态未知" || !strings.Contains(status.ProxyDetail, "下次启动使用 2345") {
		t.Fatalf("saved port and stale snapshot status = %+v", status)
	}
}

func TestSettingsSavedWithoutStartingVM(t *testing.T) {
	l := Launcher{Home: t.TempDir()}
	want := Settings{Port: "59321"}
	if err := l.SaveSettings(want); err != nil {
		t.Fatal(err)
	}
	if got := l.Settings(); got != want {
		t.Fatalf("settings = %+v; want %+v", got, want)
	}
	if err := l.SaveSettings(Settings{Port: "65536"}); err == nil || l.Settings() != want {
		t.Fatal("invalid port replaced persisted settings")
	}
}
