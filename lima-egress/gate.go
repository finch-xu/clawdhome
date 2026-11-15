package egress

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

const (
	ProbeInterval  = 10 * time.Second
	ProbeTimeout   = 3 * time.Second
	RecoveryProbes = 2
	probeLease     = ProbeInterval + ProbeTimeout
)

var ErrClosed = errors.New("VM internet access is closed")

type dialFunc func(context.Context, string, string) (net.Conn, error)

type gate struct {
	mu        sync.Mutex
	dial      dialFunc
	successes int
	expires   time.Time
	pending   map[*pendingDial]struct{}
	active    map[*trackedConn]struct{}
}

type pendingDial struct{ cancel context.CancelFunc }

func newGate(dial dialFunc) *gate {
	return &gate{dial: dial, pending: make(map[*pendingDial]struct{}), active: make(map[*trackedConn]struct{})}
}

func (g *gate) setDial(dial dialFunc) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.closeLocked()
	g.dial = dial
}

func (g *gate) probeResult(ok bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now().Round(0)
	g.expireLocked(now)
	if !ok {
		g.closeLocked()
		return
	}
	g.successes = min(g.successes+1, RecoveryProbes)
	g.expires = now.Add(probeLease)
}

func (g *gate) expire(now time.Time) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.expireLocked(now)
}

func (g *gate) expireLocked(now time.Time) bool {
	if g.expires.IsZero() || now.Before(g.expires) {
		return false
	}
	g.closeLocked()
	return true
}

func (g *gate) closeLocked() {
	g.successes = 0
	g.expires = time.Time{}
	for pending := range g.pending {
		pending.cancel()
	}
	clear(g.pending)
	for conn := range g.active {
		close(conn.revoked)
		_ = conn.Conn.Close()
	}
	clear(g.active)
}

func (g *gate) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	g.mu.Lock()
	g.expireLocked(time.Now().Round(0))
	if g.successes < RecoveryProbes || g.dial == nil {
		g.mu.Unlock()
		return nil, ErrClosed
	}
	dialCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	pending := &pendingDial{cancel: cancel}
	g.pending[pending] = struct{}{}
	dial := g.dial
	g.mu.Unlock()

	conn, err := dial(dialCtx, network, address)

	g.mu.Lock()
	defer g.mu.Unlock()
	g.expireLocked(time.Now().Round(0))
	_, current := g.pending[pending]
	delete(g.pending, pending)
	if err != nil {
		return nil, err
	}
	if !current || dialCtx.Err() != nil {
		_ = conn.Close()
		return nil, ErrClosed
	}
	tracked := &trackedConn{Conn: conn, gate: g, revoked: make(chan struct{})}
	g.active[tracked] = struct{}{}
	return tracked, nil
}

type trackedConn struct {
	net.Conn
	gate    *gate
	revoked chan struct{}
}

func (c *trackedConn) Revoked() <-chan struct{} { return c.revoked }

func (c *trackedConn) CloseRead() error {
	if conn, ok := c.Conn.(interface{ CloseRead() error }); ok {
		return conn.CloseRead()
	}
	return nil
}

func (c *trackedConn) CloseWrite() error {
	if conn, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return conn.CloseWrite()
	}
	return nil
}

func (c *trackedConn) Read(p []byte) (int, error) {
	c.gate.expire(time.Now().Round(0))
	return c.Conn.Read(p)
}

func (c *trackedConn) Write(p []byte) (int, error) {
	c.gate.expire(time.Now().Round(0))
	return c.Conn.Write(p)
}

func (c *trackedConn) Close() error {
	err := c.Conn.Close()
	c.gate.mu.Lock()
	delete(c.gate.active, c)
	c.gate.mu.Unlock()
	return err
}
