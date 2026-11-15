package vm

import (
	"net"
	"time"

	egress "github.com/finch-xu/clawdhome/lima-egress"
)

// StatusExpiry 是网关状态的过期阈值：超过这个时间没有更新，就不再相信其中的
// 代理和探测结果。
const StatusExpiry = 75 * time.Second

// SSH 通道的状态码。
const (
	SSHChecking    = "checking"
	SSHAvailable   = "available"
	SSHUnavailable = "unavailable"
	SSHNotRunning  = "not-running"
	SSHUnknown     = "unknown"
)

// VM、代理、探测和防火墙的状态码。界面按这些码查自己的文案字典，所以它们是
// 稳定的 ASCII 标识，不随语言变化。
const (
	VMRunning  = "running"
	VMStopped  = "stopped"
	VMStarting = "starting"
	VMStopping = "stopping"
	VMUnknown  = "unknown"

	ProxyWaitingCheck = "waiting-check"
	ProxyUnconfigured = "unconfigured"
	ProxyConnecting   = "connecting"
	ProxyConnected    = "connected"
	ProxyFailed       = "failed"
	ProxyDisabled     = "disabled"
	ProxyUnknown      = "unknown"

	ProbeWaiting     = "waiting"
	ProbeReachable   = "reachable"
	ProbeUnreachable = "unreachable"
	ProbeExpired     = "expired"
	ProbeSkipped     = "skipped"

	FirewallUnknown     = "unknown"
	FirewallNotRunning  = "not-running"
	FirewallBlocked     = "blocked"
	FirewallProxyOnly   = "proxy-only"
	FirewallSystemRoute = "system-route"
)

var (
	vmStatusCodes = map[string]string{
		"running": VMRunning, "stopped": VMStopped, "": VMStopped,
		"starting": VMStarting, "stopping": VMStopping,
	}
	proxyStatusCodes = map[string]string{
		egress.ProxyWaiting:   ProxyConnecting,
		egress.ProxyConnected: ProxyConnected,
		egress.ProxyFailed:    ProxyFailed,
	}
	probeStatusCodes = map[string]string{
		egress.ProbeWaiting:     ProbeWaiting,
		egress.ProbeReachable:   ProbeReachable,
		egress.ProbeUnreachable: ProbeUnreachable,
		egress.ProbeExpired:     ProbeExpired,
		egress.ProbeSkipped:     ProbeSkipped,
	}
)

// StatusView 把网关状态拆成界面要显示的几项。VM/SSH/Proxy/Probe/Firewall 是
// 状态码，由界面查字典翻译；带 Detail 后缀的字段是诊断信息（含地址、时间和
// 网关给出的原因），原样显示。
type StatusView struct {
	VM, SSH, Proxy, Probe, Firewall          string
	ProxyDetail, ProbeDetail, FirewallDetail string
}

func DescribeStatus(vmState, ssh string, cfg Settings, state *egress.GatewayStatus, now time.Time) StatusView {
	vmCode, ok := vmStatusCodes[vmState]
	if !ok {
		vmCode = VMUnknown
	}
	s := StatusView{VM: vmCode, SSH: ssh, Proxy: ProxyWaitingCheck, Probe: ProbeWaiting, Firewall: FirewallUnknown}
	if cfg.Port == "" {
		s.Proxy = ProxyUnconfigured
	} else {
		s.ProxyDetail = "已保存 " + net.JoinHostPort("127.0.0.1", cfg.Port)
	}
	if vmState == "" || vmState == "stopped" {
		s.Probe, s.Firewall = ProbeSkipped, FirewallNotRunning
		return s
	}
	if state == nil {
		s.FirewallDetail = "尚未收到当前 VM 的网关状态"
		return s
	}
	_, port, _ := net.SplitHostPort(state.ProxyAddr)
	s.ProxyDetail = "当前 " + state.ProxyAddr
	if port != cfg.Port {
		if cfg.Port == "" {
			s.ProxyDetail += " · 已清空设置，下次启动生效"
		} else {
			s.ProxyDetail += " · 下次启动使用 " + cfg.Port
		}
	}
	if now.Sub(state.UpdatedAt) > StatusExpiry {
		s.Proxy, s.Probe = ProxyUnknown, ProbeExpired
		s.FirewallDetail = "网关状态长时间未更新"
		return s
	}
	if code, ok := proxyStatusCodes[state.Proxy]; ok {
		s.Proxy = code
	}
	if state.Proxy == egress.ProxyDisabled {
		s.Proxy = ProxyDisabled
		s.ProxyDetail = "出口 " + routeLabel(state.Route)
	}
	if state.ProxyError != "" {
		s.ProxyDetail += " · " + state.ProxyError
	}
	if code, ok := probeStatusCodes[state.Probe]; ok {
		s.Probe = code
	}
	s.ProbeDetail = state.Reason
	if !state.LastProbe.IsZero() {
		s.ProbeDetail += " · 上次 " + state.LastProbe.Local().Format("01-02 15:04:05")
	}
	s.Firewall, s.FirewallDetail = FirewallBlocked, "公网新连接已阻断，已有公网连接在关断时关闭"
	if state.Online && now.Before(state.Until) {
		s.Firewall, s.FirewallDetail = FirewallProxyOnly, "仅 IPv4 TCP 和 DoH 经代理出站"
		if state.Route == egress.RouteSystem {
			s.Firewall, s.FirewallDetail = FirewallSystemRoute, "仅 IPv4 TCP 和 DoH 经宿主机系统路由出站"
		}
	} else if !state.Until.IsZero() && !now.Before(state.Until) {
		s.Probe = ProbeExpired
		s.FirewallDetail = "探测租期已过，网关按策略关闭公网连接"
	}
	return s
}
