package vm

import (
	"testing"
	"time"

	egress "github.com/finch-xu/clawdhome/lima-egress"
)

func TestStatusDistinguishesWorkingProxyFromBlockedEgress(t *testing.T) {
	now := time.Now()
	snap := egress.GatewayStatus{Running: true, ProxyAddr: "127.0.0.1:1234", Proxy: egress.ProxyConnected,
		Probe: egress.ProbeUnreachable, Reason: "Google HTTP 503", LastProbe: now, UpdatedAt: now}
	status := DescribeStatus("running", SSHAvailable, Settings{Port: "1234"}, &snap, now)
	if status.Proxy != ProxyConnected || status.Probe != ProbeUnreachable || status.Firewall != FirewallBlocked || status.ProbeReason != "Google HTTP 503" {
		t.Fatalf("status = %+v", status)
	}
	snap.Online, snap.Probe, snap.Until = true, egress.ProbeReachable, now
	status = DescribeStatus("running", SSHAvailable, Settings{Port: "1234"}, &snap, now)
	if status.Probe != ProbeExpired || status.Firewall != FirewallBlocked {
		t.Fatalf("expired status = %+v", status)
	}
	status = DescribeStatus("running", SSHAvailable, Settings{Port: "2345"}, &snap, now.Add(2*time.Minute))
	if status.Proxy != ProxyUnknown || status.Firewall != FirewallUnknown ||
		status.ProxyDetail != ProxyDetailPortPending || status.PendingPort != "2345" {
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
		got := DescribeStatus(state, SSHNotRunning, Settings{}, nil, time.Now())
		if got.Probe != ProbeSkipped || got.Firewall != FirewallNotRunning {
			t.Fatalf("vm %q: probe = %q, firewall = %q; want %q/%q", state, got.Probe, got.Firewall, ProbeSkipped, FirewallNotRunning)
		}
	}
}

func TestDescribeStatusWhenGatewayStatusMissing(t *testing.T) {
	got := DescribeStatus("running", SSHAvailable, Settings{Port: "7890"}, nil, time.Now())
	if got.FirewallDetail != FirewallDetailNoGateway {
		t.Fatalf("FirewallDetail = %q, want %q", got.FirewallDetail, FirewallDetailNoGateway)
	}
}

func TestDescribeStatusForSystemRouteEgress(t *testing.T) {
	now := time.Now()
	snap := egress.GatewayStatus{Running: true, Online: true, Route: egress.RouteSystem,
		Proxy: egress.ProxyDisabled, Probe: egress.ProbeReachable,
		UpdatedAt: now, Until: now.Add(time.Minute)}
	got := DescribeStatus("running", SSHAvailable, Settings{}, &snap, now)
	if got.Firewall != FirewallSystemRoute {
		t.Fatalf("firewall = %q, want %q", got.Firewall, FirewallSystemRoute)
	}
	if got.Proxy != ProxyDisabled {
		t.Fatalf("proxy = %q, want %q when the proxy is off", got.Proxy, ProxyDisabled)
	}
}

func TestDescribeStatusWhenProbeLeaseExpired(t *testing.T) {
	now := time.Now()
	snap := egress.GatewayStatus{Running: true, Online: true, Route: egress.RouteClash,
		Proxy: egress.ProxyConnected, Probe: egress.ProbeReachable,
		UpdatedAt: now, Until: now.Add(-time.Second)}
	got := DescribeStatus("running", SSHAvailable, Settings{}, &snap, now)
	if got.Probe != ProbeExpired {
		t.Fatalf("probe = %q, want %q once the lease is past", got.Probe, ProbeExpired)
	}
	if got.Firewall != FirewallBlocked {
		t.Fatalf("firewall = %q, want %q", got.Firewall, FirewallBlocked)
	}
}

func TestDescribeStatusReturnsCodesNotLocalizedText(t *testing.T) {
	now := time.Now()
	snap := egress.GatewayStatus{Running: true, Online: true, Route: egress.RouteClash,
		Proxy: egress.ProxyConnected, Probe: egress.ProbeReachable,
		UpdatedAt: now, Until: now.Add(time.Minute)}
	got := DescribeStatus("running", SSHAvailable, Settings{Port: "7890"}, &snap, now)

	// 状态字段是供界面查字典的稳定标识，不是某种语言的文案。
	for name, code := range map[string]string{
		"VM": got.VM, "Proxy": got.Proxy, "Probe": got.Probe, "Firewall": got.Firewall,
	} {
		if code == "" {
			t.Fatalf("%s is empty, want a status code", name)
		}
		for _, r := range code {
			if r > 127 {
				t.Fatalf("%s = %q contains non-ASCII; status fields must be codes, not localized text", name, code)
			}
		}
	}
}

func TestStatusDetailCodesCarryNoLocalizedProse(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name  string
		vm    string
		cfg   Settings
		state *egress.GatewayStatus
		at    time.Time
	}{
		{"stopped", "stopped", Settings{}, nil, now},
		{"no gateway", "running", Settings{Port: "7890"}, nil, now},
		{"stale", "running", Settings{Port: "7890"},
			&egress.GatewayStatus{Running: true, UpdatedAt: now.Add(-2 * time.Minute)}, now},
		{"blocked", "running", Settings{Port: "7890"},
			&egress.GatewayStatus{Running: true, ProxyAddr: "127.0.0.1:7890",
				Proxy: egress.ProxyConnected, Probe: egress.ProbeUnreachable, UpdatedAt: now}, now},
		{"proxy only", "running", Settings{Port: "7890"},
			&egress.GatewayStatus{Running: true, Online: true, ProxyAddr: "127.0.0.1:7890",
				Proxy: egress.ProxyConnected, Probe: egress.ProbeReachable,
				UpdatedAt: now, Until: now.Add(time.Minute)}, now},
	}
	for _, c := range cases {
		got := DescribeStatus(c.vm, SSHAvailable, c.cfg, c.state, c.at)
		for name, code := range map[string]string{
			"ProxyDetail": got.ProxyDetail, "ProbeReason": got.ProbeReason,
			"FirewallDetail": got.FirewallDetail,
		} {
			for _, r := range code {
				if r > 127 {
					t.Errorf("%s: %s = %q contains localized prose; details must be codes the ui translates",
						c.name, name, code)
					break
				}
			}
		}
	}
}
