package egress

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"

	"golang.org/x/net/proxy"
)

func validateSOCKSAddr(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	n, portErr := strconv.ParseUint(port, 10, 16)
	if err != nil || portErr != nil || n == 0 || host != "127.0.0.1" {
		return fmt.Errorf("Clash SOCKS5 endpoint must be 127.0.0.1:<port>")
	}
	return nil
}

// CheckSOCKS5 negotiates no-auth SOCKS5 without sending a destination or a CONNECT request.
func CheckSOCKS5(ctx context.Context, addr string) error {
	if err := validateSOCKSAddr(addr); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, ProbeTimeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp4", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return err
	}
	if _, err := conn.Write([]byte{5, 1, 0}); err != nil {
		return err
	}
	var reply [2]byte
	if _, err := io.ReadFull(conn, reply[:]); err != nil {
		return err
	}
	if reply != [2]byte{5, 0} {
		return fmt.Errorf("端口未接受无需认证的 SOCKS5 握手")
	}
	return nil
}

func newSOCKSDialer(addr string) (dialFunc, error) {
	if err := validateSOCKSAddr(addr); err != nil {
		return nil, err
	}
	socks, err := proxy.SOCKS5("tcp4", addr, nil, nil)
	if err != nil {
		return nil, err
	}
	// Perform negotiation on the TCP socket directly so the relay can propagate FIN.
	handshake, ok := socks.(interface {
		DialWithConn(context.Context, net.Conn, string, string) (net.Addr, error)
	})
	if !ok {
		return nil, fmt.Errorf("SOCKS5 implementation cannot negotiate on an existing TCP socket")
	}
	dialer := &net.Dialer{Timeout: ProbeTimeout}
	return func(ctx context.Context, network, destination string) (net.Conn, error) {
		conn, err := dialer.DialContext(ctx, "tcp4", addr)
		if err != nil {
			return nil, err
		}
		if _, err := handshake.DialWithConn(ctx, conn, network, destination); err != nil {
			_ = conn.Close()
			return nil, err
		}
		return conn, nil
	}, nil
}

var blockedIPv4 = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
}

func isPublicTCP(network, address string) bool {
	if network != "tcp" && network != "tcp4" {
		return false
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	addr, err := netip.ParseAddr(host)
	if err != nil || !addr.Is4() || !addr.IsGlobalUnicast() || addr.IsPrivate() {
		return false
	}
	for _, prefix := range blockedIPv4 {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}
