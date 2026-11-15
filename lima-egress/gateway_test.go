package egress

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestSOCKSGoogleFailureCutsExistingTCP(t *testing.T) {
	proxy, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()

	var googleDown atomic.Bool
	var primaryDNSDown atomic.Bool
	google := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dns-query" && r.Method == http.MethodPost {
			if r.Host == "dns.google" && primaryDNSDown.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/dns-message")
			_, _ = w.Write([]byte("test DNS response"))
			return
		}
		if r.URL.Path != "/generate_204" || googleDown.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	google.StartTLS()
	defer google.Close()
	go func() {
		for {
			conn, err := proxy.Accept()
			if err != nil {
				return
			}
			go serveTestProxy(conn, google.Listener.Addr().String())
		}
	}()

	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, RouteFile), []byte(RouteClash), 0600); err != nil {
		t.Fatal(err)
	}
	statusPath := testStatusPath(t, home)
	gateway, err := New(proxy.Addr().String(), statusPath)
	if err != nil {
		t.Fatal(err)
	}
	tlsConfig := google.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	tlsConfig.ServerName = "127.0.0.1"
	gateway.probe.Transport.(*http.Transport).TLSClientConfig = tlsConfig
	gateway.dns.Transport.(*http.Transport).TLSClientConfig = tlsConfig

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	go gateway.Run(ctx)
	startupLimit := time.Now().Add(probeLease + time.Second)
	for {
		state := readTestStatus(statusPath)
		if state.Online && state.Proxy == ProxyConnected && state.Probe == ProbeReachable && !state.LastProbe.IsZero() {
			break
		}
		if time.Now().After(startupLimit) {
			t.Fatal("two scheduled successes did not open VM egress")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if result, err := gateway.ResolveDNS(ctx, []byte("test DNS query")); err != nil || string(result) != "test DNS response" {
		t.Fatalf("DoH through SOCKS = %q, error = %v", result, err)
	}
	primaryDNSDown.Store(true)
	if result, err := gateway.ResolveDNS(ctx, []byte("test DNS query")); err != nil || string(result) != "test DNS response" {
		t.Fatalf("fallback DoH through SOCKS = %q, error = %v", result, err)
	}
	fin, err := gateway.DialTCP(ctx, "tcp", "1.0.0.1:443")
	if err != nil {
		t.Fatal(err)
	}
	defer fin.Close()
	_ = fin.SetDeadline(time.Now().Add(time.Second))
	if _, err := fin.Write([]byte("one request")); err != nil {
		t.Fatal(err)
	}
	if err := fin.(*trackedConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	if response, err := io.ReadAll(fin); err != nil || string(response) != "FIN received" {
		t.Fatalf("SOCKS half-close response = %q, error = %v", response, err)
	}

	conn, err := gateway.DialTCP(ctx, "tcp", "1.1.1.1:443")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("live")); err != nil {
		t.Fatal(err)
	}
	echo := make([]byte, 4)
	if _, err := io.ReadFull(conn, echo); err != nil || string(echo) != "live" {
		t.Fatalf("echo = %q, error = %v", echo, err)
	}
	_ = conn.SetDeadline(time.Time{})

	googleDown.Store(true)
	injected := time.Now()
	select {
	case <-conn.(*trackedConn).Revoked():
		t.Logf("closed existing TCP %s after locally injecting probe failure", time.Since(injected).Round(time.Millisecond))
	case <-time.After(probeLease + time.Second):
		t.Fatal("scheduled probe did not revoke the existing TCP")
	}
	if _, err := conn.Write([]byte("blocked")); err == nil {
		t.Fatal("existing TCP connection survived Google probe failure")
	}
	if _, err := gateway.DialTCP(ctx, "tcp", "1.1.1.1:443"); err != ErrClosed {
		t.Fatalf("new TCP after probe failure: %v", err)
	}
	if _, err := gateway.ResolveDNS(ctx, []byte("test DNS query")); err == nil {
		t.Fatal("DoH request succeeded after probe failure")
	}
	deadline := time.Now().Add(time.Second)
	for {
		state := readTestStatus(statusPath)
		if !state.Online && state.Proxy == ProxyConnected && state.Probe == ProbeUnreachable && state.LastProbe.After(injected) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("blocked status did not distinguish the working proxy from failed Google probe: %+v", state)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func testStatusPath(t *testing.T, home string) string {
	t.Helper()
	dir := filepath.Join(home, "cli-mvp")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "mvp-network.json")
}

func readTestStatus(path string) GatewayStatus {
	var status GatewayStatus
	data, _ := os.ReadFile(path)
	_ = json.Unmarshal(data, &status)
	return status
}

func serveTestProxy(conn net.Conn, google string) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	greeting := make([]byte, 3)
	if _, err := io.ReadFull(conn, greeting); err != nil || greeting[0] != 5 {
		return
	}
	if _, err := conn.Write([]byte{5, 0}); err != nil {
		return
	}
	head := make([]byte, 4)
	if _, err := io.ReadFull(conn, head); err != nil || head[0] != 5 || head[1] != 1 {
		return
	}
	host := make([]byte, 4)
	if head[3] == 3 {
		var n [1]byte
		if _, err := io.ReadFull(conn, n[:]); err != nil {
			return
		}
		host = make([]byte, n[0])
	} else if head[3] != 1 {
		return
	}
	var port [2]byte
	if _, err := io.ReadFull(conn, host); err != nil {
		return
	}
	if _, err := io.ReadFull(conn, port[:]); err != nil {
		return
	}
	if _, err := conn.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0}); err != nil {
		return
	}
	switch string(host) {
	case "www.google.com", "dns.google", "cloudflare-dns.com":
		upstream, err := net.DialTimeout("tcp", google, time.Second)
		if err != nil {
			return
		}
		defer upstream.Close()
		go func() { _, _ = io.Copy(upstream, conn) }()
		_, _ = io.Copy(conn, upstream)
	case "\x01\x01\x01\x01":
		_, _ = io.Copy(conn, conn)
	case "\x01\x00\x00\x01":
		if _, err := io.ReadAll(conn); err == nil {
			_, _ = conn.Write([]byte("FIN received"))
		}
	}
}
