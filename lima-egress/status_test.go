package egress

import (
	"context"
	"io"
	"net"
	"testing"
	"time"
)

func TestCheckSOCKS5OnlyNegotiates(t *testing.T) {
	for _, method := range []byte{0, 255} {
		ln, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		done := make(chan []byte, 1)
		go func() {
			conn, err := ln.Accept()
			if err != nil {
				done <- nil
				return
			}
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(time.Second))
			var greeting [3]byte
			_, _ = io.ReadFull(conn, greeting[:])
			_, _ = conn.Write([]byte{5, method})
			rest, _ := io.ReadAll(conn)
			done <- append(greeting[:], rest...)
		}()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err = CheckSOCKS5(ctx, ln.Addr().String())
		cancel()
		if (err == nil) != (method == 0) {
			t.Fatalf("SOCKS5 reply method %d: %v", method, err)
		}
		if got := <-done; string(got) != "\x05\x01\x00" {
			t.Fatalf("port test sent %x, want only SOCKS5 greeting", got)
		}
	}
}
