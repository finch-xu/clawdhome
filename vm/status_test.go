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

func TestDescribeStatusWhenVMNotRunning(t *testing.T) {
	for _, state := range []string{"", "stopped"} {
		got := DescribeStatus(state, "未运行", Settings{}, nil, time.Now())
		if got.Probe != "未探测" || got.Firewall != "未运行" {
			t.Fatalf("vm %q: probe = %q, firewall = %q; want 未探测/未运行", state, got.Probe, got.Firewall)
		}
	}
}

func TestDescribeStatusWhenGatewayStatusMissing(t *testing.T) {
	got := DescribeStatus("running", "可用", Settings{Port: "7890"}, nil, time.Now())
	if got.FirewallDetail != "尚未收到当前 VM 的网关状态" {
		t.Fatalf("firewallDetail = %q, want the missing-gateway note", got.FirewallDetail)
	}
}

func TestDescribeStatusForSystemRouteEgress(t *testing.T) {
	now := time.Now()
	snap := egress.GatewayStatus{Running: true, Online: true, Route: egress.RouteSystem,
		Proxy: egress.ProxyDisabled, Probe: egress.ProbeReachable,
		UpdatedAt: now, Until: now.Add(time.Minute)}
	got := DescribeStatus("running", "可用", Settings{}, &snap, now)
	if got.Firewall != "系统路由出站" {
		t.Fatalf("firewall = %q, want 系统路由出站", got.Firewall)
	}
	if got.Proxy != "已停用" {
		t.Fatalf("proxy = %q, want 已停用 when the proxy is off", got.Proxy)
	}
}

func TestDescribeStatusWhenProbeLeaseExpired(t *testing.T) {
	now := time.Now()
	snap := egress.GatewayStatus{Running: true, Online: true, Route: egress.RouteClash,
		Proxy: egress.ProxyConnected, Probe: egress.ProbeReachable,
		UpdatedAt: now, Until: now.Add(-time.Second)}
	got := DescribeStatus("running", "可用", Settings{}, &snap, now)
	if got.Probe != "结果过期" {
		t.Fatalf("probe = %q, want 结果过期 once the lease is past", got.Probe)
	}
	if got.Firewall != "外网已阻断" {
		t.Fatalf("firewall = %q, want 外网已阻断", got.Firewall)
	}
}
