package iplookup

import (
	"context"
	"errors"
	"net"
	"strconv"
	"syscall"
	"time"
)

func probeDial(ctx context.Context, ip string, port int) (net.Conn, error) {
	return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
}
func (s *EnhancedService) probe(ctx context.Context, ip string, port int, key string, c Config, milestone func(int) error) probeOutcome {
	if key == "tcp" {
		conn, err := probeDial(ctx, ip, port)
		if err == nil {
			_ = conn.Close()
			return probeOutcome{status: "success"}
		}
		if errors.Is(err, syscall.ECONNREFUSED) {
			return probeOutcome{status: "closed"}
		}
		return probeOutcome{status: "error"}
	}
	if key == "tls" {
		return s.probeTLS(ctx, ip, port, c)
	}
	if len(key) > 7 && key[:7] == "trojan_" {
		return trojanProbe(ctx, ip, port, key, milestone)
	}
	if key == "shadowsocks" {
		return shadowsocksProbe(ctx, ip, port)
	}
	return handshakeProbe(ctx, ip, port, key)
}
