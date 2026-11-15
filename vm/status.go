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

// 代理和防火墙详情的状态码。
const (
	ProxyDetailNone        = ""
	ProxyDetailSavedPort   = "saved-port"
	ProxyDetailCurrentAddr = "current-addr"
	ProxyDetailPortCleared = "port-cleared"
	ProxyDetailPortPending = "port-pending"
	ProxyDetailEgressRoute = "egress-route"

	FirewallDetailNone        = ""
	FirewallDetailNoGateway   = "no-gateway"
	FirewallDetailStale       = "stale"
	FirewallDetailBlocked     = "blocked"
	FirewallDetailProxyOnly   = "proxy-only"
	FirewallDetailSystemRoute = "system-route"
	FirewallDetailLeaseExpiry = "lease-expiry"
)

// StatusView 把网关状态拆成界面要显示的几项。所有状态和详情都是状态码，由界面
// 查字典翻译；带数据的字段（地址、端口、时间）供界面套进文案模板。网关自己给出
// 的错误原文放在 ProxyError/ProbeReason 里，原样显示——它们是诊断信息，和
// curl 的报错一样不随界面语言变化。
type StatusView struct {
	VM, SSH, Proxy, Probe, Firewall string
	// ProxyDetail/FirewallDetail 是详情的状态码。
	ProxyDetail, FirewallDetail string
	// SavedProxyAddr 是设置里存的代理地址，CurrentProxyAddr 是网关当前在用的。
	SavedProxyAddr, CurrentProxyAddr string
	// PendingPort 是下次启动才生效的端口。
	PendingPort string
	// EgressRoute 是代理停用时实际走的出口名。
	EgressRoute string
	// ProxyError 和 ProbeReason 是网关给出的原文。
	ProxyError, ProbeReason string
	ProbeReasonCode         string
	ProbeHTTPStatus         int
	// LastProbe 是最近一次探测的时间，零值表示还没探测过。
	LastProbe time.Time
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
		s.ProxyDetail, s.SavedProxyAddr = ProxyDetailSavedPort, net.JoinHostPort("127.0.0.1", cfg.Port)
	}
	if vmState == "" || vmState == "stopped" {
		s.Probe, s.Firewall = ProbeSkipped, FirewallNotRunning
		return s
	}
	if state == nil {
		s.FirewallDetail = FirewallDetailNoGateway
		return s
	}
	_, port, _ := net.SplitHostPort(state.ProxyAddr)
	s.ProxyDetail, s.CurrentProxyAddr = ProxyDetailCurrentAddr, state.ProxyAddr
	if port != cfg.Port {
		if cfg.Port == "" {
			s.ProxyDetail = ProxyDetailPortCleared
		} else {
			s.ProxyDetail, s.PendingPort = ProxyDetailPortPending, cfg.Port
		}
	}
	s.ProbeReason, s.LastProbe = state.Reason, state.LastProbe
	if now.Sub(state.UpdatedAt) > StatusExpiry {
		s.ProbeReasonCode = egress.ReasonExpired
		s.Proxy, s.Probe = ProxyUnknown, ProbeExpired
		s.FirewallDetail = FirewallDetailStale
		return s
	}
	if code, ok := proxyStatusCodes[state.Proxy]; ok {
		s.Proxy = code
	}
	if state.Proxy == egress.ProxyDisabled {
		s.Proxy = ProxyDisabled
		s.ProxyDetail, s.EgressRoute = ProxyDetailEgressRoute, string(state.Route)
	}
	s.ProxyError = state.ProxyError
	if code, ok := probeStatusCodes[state.Probe]; ok {
		s.Probe = code
	}
	s.ProbeReasonCode, s.ProbeHTTPStatus = state.ReasonCode, state.ProbeHTTPStatus
	s.Firewall, s.FirewallDetail = FirewallBlocked, FirewallDetailBlocked
	if state.Online && now.Before(state.Until) {
		s.Firewall, s.FirewallDetail = FirewallProxyOnly, FirewallDetailProxyOnly
		if state.Route == egress.RouteSystem {
			s.Firewall, s.FirewallDetail = FirewallSystemRoute, FirewallDetailSystemRoute
		}
	} else if !state.Until.IsZero() && !now.Before(state.Until) {
		s.Probe, s.ProbeReasonCode, s.ProbeHTTPStatus = ProbeExpired, egress.ReasonExpired, 0
		s.FirewallDetail = FirewallDetailLeaseExpiry
	}
	return s
}
