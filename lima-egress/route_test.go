package egress

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRouteChangeRevokesConnectionsAndRequiresFreshProbes(t *testing.T) {
	client, peer := net.Pipe()
	defer peer.Close()
	g := newGate(func(context.Context, string, string) (net.Conn, error) { return client, nil })
	for range RecoveryProbes {
		g.probeResult(true)
	}
	conn, err := g.DialContext(context.Background(), "tcp4", "1.1.1.1:443")
	if err != nil {
		t.Fatal(err)
	}
	g.setDial(nil)
	select {
	case <-conn.(*trackedConn).Revoked():
	default:
		t.Fatal("old connection survived a route switch")
	}
	if _, err := g.DialContext(context.Background(), "tcp4", "1.1.1.1:443"); !errors.Is(err, ErrClosed) {
		t.Fatalf("route change did not close gate: %v", err)
	}
}

type probeTransport func(*http.Request) (*http.Response, error)

func (f probeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGatewayAppliesSystemAndBlockedRoutesWithoutRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, RouteFile)
	statusPath := testStatusPath(t, dir)
	if err := os.WriteFile(path, []byte(RouteSystem), 0600); err != nil {
		t.Fatal(err)
	}
	g, err := New("", statusPath)
	if err != nil {
		t.Fatal(err)
	}
	binaryProbe := make(chan struct{}, 1)
	g.systemProbe.Transport = probeTransport(func(*http.Request) (*http.Response, error) {
		binaryProbe <- struct{}{}
		return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	go g.Run(ctx)
	select {
	case <-binaryProbe:
	case <-ctx.Done():
		t.Fatal("system route did not use its probe without a Clash port")
	}
	if err := os.WriteFile(path, []byte(RouteBlocked), 0600); err != nil {
		t.Fatal(err)
	}
	for ctx.Err() == nil {
		s := readTestStatus(statusPath)
		if s.Route == RouteBlocked && s.Proxy == ProxyDisabled && !s.Online && strings.Contains(s.Reason, "断开") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("gateway did not apply manual internet cutoff")
}

func TestReadRouteDefaultsToSystemAndInvalidFileBlocks(t *testing.T) {
	dir := t.TempDir()
	if got := ReadRoute(dir); got != RouteSystem {
		t.Fatalf("missing route = %q, want %q", got, RouteSystem)
	}
	if _, err := New("", testStatusPath(t, dir)); err != nil {
		t.Fatalf("default route must start without a Clash port: %v", err)
	}
	for _, tc := range []struct{ file, want Route }{{RouteClash, RouteClash}, {RouteBlocked, RouteBlocked}, {"unknown", RouteBlocked}} {
		if err := os.WriteFile(filepath.Join(dir, RouteFile), []byte(tc.file), 0600); err != nil {
			t.Fatal(err)
		}
		if got := ReadRoute(dir); got != tc.want {
			t.Fatalf("route = %q, want %q", got, tc.want)
		}
	}
}
