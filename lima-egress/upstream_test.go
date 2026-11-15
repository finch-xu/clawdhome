package egress

import (
	"net"
	"strings"
	"testing"
)

func TestOnlyPublicIPv4TCPDestinationsAreAccepted(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:443", "192.168.5.2:53", "10.0.0.1:80", "100.64.0.2:80", "169.254.169.254:80", "0.0.0.0:80", "[2001:4860:4860::8888]:443", "localhost:443"} {
		if isPublicTCP("tcp", addr) {
			t.Errorf("allowed excluded address %s", addr)
		}
	}
	if isPublicTCP("udp", "8.8.8.8:53") || !isPublicTCP("tcp", "8.8.8.8:443") {
		t.Fatal("expected public IPv4 TCP only")
	}
}

func TestProxyAddressMustBeHostLoopback(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:7890", "1.1.1.1:7890", "localhost:7890", "127.0.0.1:0", "127.0.0.1:http", "127.0.0.1:65536"} {
		if _, err := newSOCKSDialer(addr); err == nil {
			t.Errorf("allowed invalid proxy address %s", addr)
		}
	}
	_, err := newSOCKSDialer(net.JoinHostPort("127.0.0.1", "7890"))
	if err != nil {
		t.Fatal(strings.TrimSpace(err.Error()))
	}
}
