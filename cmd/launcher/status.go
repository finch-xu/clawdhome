package main

import (
	"fmt"
	"net"
	"time"

	egress "github.com/finch-xu/clawdhome/lima-egress"
)

type statusView struct {
	vm, proxy, probe, firewall                         string
	vmDetail, proxyDetail, probeDetail, firewallDetail string
}

func describeStatus(vm, ssh string, cfg settings, state *egress.GatewayStatus, now time.Time) statusView {
	vmLabel := map[string]string{"running": "运行中", "stopped": "已停止", "": "已停止", "starting": "启动中", "stopping": "停止中"}[vm]
	if vmLabel == "" {
		vmLabel = "状态未知"
	}
	s := statusView{vm: vmLabel, vmDetail: "本地 SSH " + ssh, proxy: "等待检测", probe: "等待探测", firewall: "状态未知"}
	if cfg.Port == "" {
		s.proxy = "未配置"
	} else {
		s.proxyDetail = "已保存 " + net.JoinHostPort("127.0.0.1", cfg.Port)
	}
	if vm == "" || vm == "stopped" {
		s.probe, s.firewall = "未探测", "未运行"
		return s
	}
	if state == nil {
		s.firewallDetail = "尚未收到当前 VM 的网关状态"
		return s
	}
	_, port, _ := net.SplitHostPort(state.ProxyAddr)
	s.proxyDetail = "当前 " + state.ProxyAddr
	if port != cfg.Port {
		if cfg.Port == "" {
			s.proxyDetail += " · 已清空设置，下次启动生效"
		} else {
			s.proxyDetail += " · 下次启动使用 " + cfg.Port
		}
	}
	if now.Sub(state.UpdatedAt) > 75*time.Second {
		s.proxy, s.probe = "状态未知", "结果过期"
		s.firewallDetail = "网关状态长时间未更新"
		return s
	}
	s.proxy = map[string]string{egress.ProxyWaiting: "连接中", egress.ProxyConnected: "已连接", egress.ProxyFailed: "连接失败"}[state.Proxy]
	if state.Proxy == egress.ProxyDisabled {
		s.proxy = "已停用"
		s.proxyDetail = "出口 " + routeLabel(state.Route)
	}
	if state.ProxyError != "" {
		s.proxyDetail += " · " + state.ProxyError
	}
	s.probe = map[string]string{egress.ProbeWaiting: "等待探测", egress.ProbeReachable: "可达", egress.ProbeUnreachable: "探测失败", egress.ProbeExpired: "结果过期", egress.ProbeSkipped: "未探测"}[state.Probe]
	s.probeDetail = state.Reason
	if !state.LastProbe.IsZero() {
		s.probeDetail += " · 上次 " + state.LastProbe.Local().Format("01-02 15:04:05")
	}
	s.firewall, s.firewallDetail = "外网已阻断", "公网新连接已阻断，已有公网连接在关断时关闭"
	if state.Online && now.Before(state.Until) {
		s.firewall, s.firewallDetail = "仅允许经代理出站", "仅 IPv4 TCP 和 DoH 经代理出站"
		if state.Route == egress.RouteSystem {
			s.firewall, s.firewallDetail = "系统路由出站", "仅 IPv4 TCP 和 DoH 经宿主机系统路由出站"
		}
	} else if !state.Until.IsZero() && !now.Before(state.Until) {
		s.probe = "结果过期"
		s.firewallDetail = "探测租期已过，网关按策略关闭公网连接"
	}
	return s
}

func (s statusView) summary() string {
	return fmt.Sprintf("VM %s  ·  Clash %s  ·  Google %s  ·  防火墙 %s", s.vm, s.proxy, s.probe, s.firewall)
}
