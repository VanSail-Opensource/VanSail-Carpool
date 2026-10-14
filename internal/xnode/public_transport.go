package xnode

import (
	"context"
	"net"
	"net/http"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func publicTransport() *http.Transport {
	return &http.Transport{Proxy: nil, DisableKeepAlives: true, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 10 * time.Second, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, ErrInvalid
		}
		addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(addresses) == 0 {
			return nil, ErrUnavailable
		}
		for _, address := range addresses {
			if !wire.PublicProxyAddress(address.IP.String()) {
				return nil, ErrInvalid
			}
		}
		dialer := net.Dialer{Timeout: 10 * time.Second}
		var last error
		for _, address := range addresses {
			connection, err := dialer.DialContext(ctx, network, net.JoinHostPort(address.IP.String(), port))
			if err == nil {
				return connection, nil
			}
			last = err
		}
		return nil, last
	}}
}
