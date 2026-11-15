package vm

import (
	"fmt"
	"net"
	"time"

	egress "github.com/finch-xu/clawdhome/lima-egress"
)

// StatusExpiry 是网关状态的过期阈值：超过这个时间没有更新，就不再相信其中的
// 代理和探测结果。
const StatusExpiry = 75 * time.Second

var (
	vmStatusLabels = map[string]string{
		"running": "运行中", "stopped": "已停止", "": "已停止",
		"starting": "启动中", "stopping": "停止中",
	}
	proxyStatusLabels = map[string]string{
		egress.ProxyWaiting:   "连接中",
		egress.ProxyConnected: "已连接",
		egress.ProxyFailed:    "连接失败",
	}
	probeStatusLabels = map[string]string{
		egress.ProbeWaiting:     "等待探测",
		egress.ProbeReachable:   "可达",
		egress.ProbeUnreachable: "探测失败",
		egress.ProbeExpired:     "结果过期",
		egress.ProbeSkipped:     "未探测",
	}
)

// StatusView 是状态条要显示的四项状态及各自的详情文案。
type StatusView struct {
	VM, Proxy, Probe, Firewall                         string
	VMDetail, ProxyDetail, ProbeDetail, FirewallDetail string
}

func DescribeStatus(vm, ssh string, cfg Settings, state *egress.GatewayStatus, now time.Time) StatusView {
	vmLabel := vmStatusLabels[vm]
	if vmLabel == "" {
		vmLabel = "状态未知"
	}
	s := StatusView{VM: vmLabel, VMDetail: "本地 SSH " + ssh, Proxy: "等待检测", Probe: "等待探测", Firewall: "状态未知"}
	if cfg.Port == "" {
		s.Proxy = "未配置"
	} else {
		s.ProxyDetail = "已保存 " + net.JoinHostPort("127.0.0.1", cfg.Port)
	}
	if vm == "" || vm == "stopped" {
		s.Probe, s.Firewall = "未探测", "未运行"
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
		s.Proxy, s.Probe = "状态未知", "结果过期"
		s.FirewallDetail = "网关状态长时间未更新"
		return s
	}
	s.Proxy = proxyStatusLabels[state.Proxy]
	if state.Proxy == egress.ProxyDisabled {
		s.Proxy = "已停用"
		s.ProxyDetail = "出口 " + routeLabel(state.Route)
	}
	if state.ProxyError != "" {
		s.ProxyDetail += " · " + state.ProxyError
	}
	s.Probe = probeStatusLabels[state.Probe]
	s.ProbeDetail = state.Reason
	if !state.LastProbe.IsZero() {
		s.ProbeDetail += " · 上次 " + state.LastProbe.Local().Format("01-02 15:04:05")
	}
	s.Firewall, s.FirewallDetail = "外网已阻断", "公网新连接已阻断，已有公网连接在关断时关闭"
	if state.Online && now.Before(state.Until) {
		s.Firewall, s.FirewallDetail = "仅允许经代理出站", "仅 IPv4 TCP 和 DoH 经代理出站"
		if state.Route == egress.RouteSystem {
			s.Firewall, s.FirewallDetail = "系统路由出站", "仅 IPv4 TCP 和 DoH 经宿主机系统路由出站"
		}
	} else if !state.Until.IsZero() && !now.Before(state.Until) {
		s.Probe = "结果过期"
		s.FirewallDetail = "探测租期已过，网关按策略关闭公网连接"
	}
	return s
}

func (s StatusView) Summary() string {
	return fmt.Sprintf("VM %s  ·  Clash %s  ·  Google %s  ·  防火墙 %s", s.VM, s.Proxy, s.Probe, s.Firewall)
}
