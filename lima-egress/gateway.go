package egress

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	probeURL       = "https://www.google.com/generate_204"
	dnsURL         = "https://dns.google/dns-query"
	fallbackDNSURL = "https://cloudflare-dns.com/dns-query"
)

type Gateway struct {
	gate        *gate
	probe       *http.Client
	dns         *http.Client
	status      string
	proxyAddr   string
	clashDial   dialFunc
	systemDial  dialFunc
	systemProbe *http.Client
	routeDir    string
}

// New receives the status path in the Lima instance directory. The route
// preference lives one level up in Lima home, alongside launcher settings.
func New(proxyAddr, statusFile string) (*Gateway, error) {
	var routeDir string
	if statusFile != "" {
		routeDir = filepath.Dir(filepath.Dir(statusFile))
	}
	dial, err := newSOCKSDialer(proxyAddr)
	if err != nil && ReadRoute(routeDir) == RouteClash {
		return nil, err
	}
	system := &net.Dialer{Timeout: ProbeTimeout}
	systemDial := func(ctx context.Context, _, address string) (net.Conn, error) {
		return system.DialContext(ctx, "tcp4", address)
	}
	g := newGate(dial)
	var probe *http.Client
	if dial != nil {
		probe = newHTTPClient(dial)
	}
	return &Gateway{gate: g, probe: probe, dns: newHTTPClient(g.DialContext), status: statusFile, proxyAddr: proxyAddr,
		clashDial: dial, systemDial: systemDial, systemProbe: newHTTPClient(systemDial), routeDir: routeDir}, nil
}

func newHTTPClient(dial dialFunc) *http.Client {
	return &http.Client{
		Transport: &http.Transport{DialContext: dial, DisableKeepAlives: true},
		Timeout:   ProbeTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (g *Gateway) DialTCP(ctx context.Context, network, address string) (net.Conn, error) {
	if !isPublicTCP(network, address) {
		return nil, fmt.Errorf("VM destination is outside the permitted IPv4 TCP scope")
	}
	return g.gate.DialContext(ctx, network, address)
}

func (g *Gateway) ResolveDNS(ctx context.Context, query []byte) ([]byte, error) {
	ctx, stop := context.WithTimeout(ctx, ProbeTimeout)
	defer stop()
	primaryCtx, cancel := context.WithTimeout(ctx, ProbeTimeout/2)
	answer, err := g.resolveDoH(primaryCtx, dnsURL, query)
	cancel()
	if err == nil || errors.Is(err, ErrClosed) || ctx.Err() != nil {
		return answer, err
	}
	return g.resolveDoH(ctx, fallbackDNSURL, query)
}

func (g *Gateway) resolveDoH(ctx context.Context, endpoint string, query []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(query))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")
	resp, err := g.dns.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "application/dns-message" {
		return nil, fmt.Errorf("unexpected DoH response: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 65536))
}

func (g *Gateway) Run(ctx context.Context) {
	state := g.applyRoute(ReadRoute(g.routeDir))
	cancelProbe := func() {}
	defer func() { cancelProbe() }()
	var generation uint64
	timer := time.NewTimer(0)
	defer timer.Stop()
	if state.Route == RouteBlocked {
		timer.Stop()
	}
	defer func() {
		g.gate.probeResult(false)
		state.Running, state.Reason, state.ReasonCode, state.ProbeHTTPStatus = false, "网关已停止", ReasonStopped, 0
		g.publish(state)
	}()
	checkLease := time.NewTicker(250 * time.Millisecond)
	defer checkLease.Stop()
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()
	probes := make(chan probeOutcome, 1)
	syncRoute := func() bool {
		route := ReadRoute(g.routeDir)
		if route == state.Route {
			return false
		}
		cancelProbe()
		generation++
		state = g.applyRoute(route)
		timer.Stop()
		if route != RouteBlocked {
			timer.Reset(0)
		}
		return true
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if state.Route == RouteBlocked {
				g.publish(state)
			}
		case <-checkLease.C:
			if syncRoute() {
				continue
			}
			if g.gate.expire(time.Now().Round(0)) {
				state.Probe, state.Reason, state.ReasonCode, state.ProbeHTTPStatus = ProbeExpired, "Google 探测已过期", ReasonExpired, 0
				g.publish(state)
			}
		case <-timer.C:
			if syncRoute() {
				continue
			}
			cancelProbe = g.launchProbe(ctx, state.Route, generation, probes)
		case outcome := <-probes:
			if outcome.generation != generation {
				continue
			}
			if syncRoute() {
				continue
			}
			// A probe that straddled host sleep cannot reopen the gate.
			if time.Since(outcome.started.Round(0)) > ProbeTimeout+time.Second {
				outcome.ok = false
				outcome.probe, outcome.reason, outcome.reasonCode, outcome.httpStatus = ProbeExpired, "Google 探测已过期", ReasonExpired, 0
			}
			g.gate.probeResult(outcome.ok)
			state.Proxy, state.ProxyError = outcome.proxy, outcome.proxyError
			state.Probe, state.Reason = outcome.probe, outcome.reason
			state.ReasonCode, state.ProbeHTTPStatus = outcome.reasonCode, outcome.httpStatus
			if !outcome.lastProbe.IsZero() {
				state.LastProbe = outcome.lastProbe
			}
			g.publish(state)
			timer.Reset(outcome.delay)
		}
	}
}

func (g *Gateway) launchProbe(ctx context.Context, route Route, generation uint64, results chan<- probeOutcome) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()
		outcome := g.check(ctx, route)
		outcome.generation = generation
		select {
		case results <- outcome:
		case <-ctx.Done():
		}
	}()
	return cancel
}

func (g *Gateway) applyRoute(route Route) GatewayStatus {
	state := GatewayStatus{PID: os.Getpid(), Running: true, Route: route, ProxyAddr: g.proxyAddr,
		Proxy: ProxyDisabled, Probe: ProbeWaiting, Reason: "等待 Google 探测", ReasonCode: ReasonWaiting}
	switch route {
	case RouteClash:
		state.Proxy = ProxyWaiting
		g.gate.setDial(g.clashDial)
	case RouteSystem:
		g.gate.setDial(g.systemDial)
	default:
		g.gate.setDial(nil)
		state.Probe, state.Reason, state.ReasonCode = ProbeSkipped, "已手动断开公网", ReasonBlocked
	}
	g.publish(state)
	return state
}

type probeOutcome struct {
	started    time.Time
	generation uint64
	lastProbe  time.Time
	ok         bool
	proxy      string
	proxyError string
	probe      string
	reason     string
	reasonCode string
	httpStatus int
	delay      time.Duration
}

func (g *Gateway) check(ctx context.Context, route Route) (outcome probeOutcome) {
	now := time.Now()
	outcome = probeOutcome{started: now}
	defer func() {
		if outcome.delay == 0 {
			outcome.delay = max(time.Millisecond, ProbeInterval-time.Since(now))
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, ProbeTimeout)
	defer cancel()
	client := g.systemProbe
	outcome.proxy, outcome.probe = ProxyDisabled, ProbeUnreachable
	if route == RouteClash {
		if err := CheckSOCKS5(ctx, g.proxyAddr); err != nil || g.probe == nil {
			outcome.proxy = ProxyFailed
			if err != nil {
				outcome.proxyError = err.Error()
			}
			outcome.probe, outcome.reason, outcome.reasonCode = ProbeSkipped, "Clash SOCKS5 不可用", ReasonProxyFailed
			return outcome
		}
		client, outcome.proxy = g.probe, ProxyConnected
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		outcome.reason, outcome.reasonCode = "Google 探测请求无效", ReasonRequest
		return outcome
	}
	resp, err := client.Do(req)
	outcome.lastProbe = time.Now()
	if err != nil {
		outcome.reason, outcome.reasonCode = "当前出口的 Google HTTPS 探测失败", ReasonHTTPSFailed
		return outcome
	}
	_ = resp.Body.Close()
	outcome.ok = resp.StatusCode == http.StatusNoContent
	if outcome.ok {
		outcome.probe = ProbeReachable
	}
	outcome.reason = "Google HTTP " + strconv.Itoa(resp.StatusCode)
	outcome.reasonCode, outcome.httpStatus = ReasonHTTPStatus, resp.StatusCode
	if resp.StatusCode == http.StatusTooManyRequests {
		outcome.delay = time.Minute
	}
	return outcome
}

func (g *Gateway) publish(state GatewayStatus) {
	if g.status == "" {
		return
	}
	g.gate.mu.Lock()
	state.Online = g.gate.successes >= RecoveryProbes
	state.Until = g.gate.expires
	g.gate.mu.Unlock()
	state.UpdatedAt = time.Now()
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	tmp := g.status + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err == nil {
		_ = os.Rename(tmp, g.status)
	}
}
