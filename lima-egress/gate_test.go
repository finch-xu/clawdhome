package egress

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestFailureClosesConnectionsAndRequiresTwoSuccesses(t *testing.T) {
	client, peer := net.Pipe()
	defer peer.Close()
	gate := newGate(func(context.Context, string, string) (net.Conn, error) {
		return client, nil
	})
	ctx := context.Background()
	if _, err := gate.DialContext(ctx, "tcp", "1.1.1.1:443"); !errors.Is(err, ErrClosed) {
		t.Fatalf("startup dial error = %v, want closed gate", err)
	}
	gate.probeResult(true)
	if _, err := gate.DialContext(ctx, "tcp", "1.1.1.1:443"); !errors.Is(err, ErrClosed) {
		t.Fatalf("dial after one success = %v, want closed gate", err)
	}
	gate.probeResult(true)
	if _, err := gate.DialContext(ctx, "tcp", "1.1.1.1:443"); err != nil {
		t.Fatalf("dial after two successes: %v", err)
	}
	gate.probeResult(false)
	_ = peer.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := peer.Read(make([]byte, 1)); err == nil || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("peer read after failure = %v, want closed connection", err)
	} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Fatalf("connection stayed open: %v", err)
	}
	if _, err := gate.DialContext(ctx, "tcp", "1.1.1.1:443"); !errors.Is(err, ErrClosed) {
		t.Fatalf("dial after failure = %v, want closed gate", err)
	}
}

func TestPendingDialIsCanceledWhenProbeFails(t *testing.T) {
	started := make(chan struct{})
	gate := newGate(func(ctx context.Context, _, _ string) (net.Conn, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	gate.probeResult(true)
	gate.probeResult(true)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := gate.DialContext(ctx, "tcp", "1.1.1.1:443")
		result <- err
	}()
	<-started
	gate.probeResult(false)
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("pending dial error = %v, want context.Canceled", err)
	}
}

func TestExpireClosesGate(t *testing.T) {
	gate := newGate(func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("expired gate attempted an upstream dial")
		return nil, nil
	})
	gate.probeResult(true)
	gate.probeResult(true)
	if !gate.expire(time.Now().Add(time.Minute)) {
		t.Fatal("stale probe did not expire")
	}
	if _, err := gate.DialContext(context.Background(), "tcp", "1.1.1.1:443"); !errors.Is(err, ErrClosed) {
		t.Fatalf("dial after lease expiry = %v, want closed gate", err)
	}
}
