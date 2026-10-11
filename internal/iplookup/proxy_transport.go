package iplookup

import (
	"context"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/platform/upstreamqueue"
)

func (s *EnhancedService) downloadInventory(ctx context.Context, raw string, c Config) ([]byte, error) {
	return upstreamqueue.Do(ctx, s.queues["proxy_lists"], func(ctx context.Context) ([]byte, error) {
		callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, e := net.SplitHostPort(address)
			if e != nil {
				return nil, e
			}
			ips, e := s.resolve(ctx, host, c.DNSResolvers, false)
			if e != nil {
				return nil, e
			}
			if len(ips) == 0 {
				return nil, &CodeError{"IP_LOOKUP_PROVIDER_UNAVAILABLE"}
			}
			for _, ip := range ips {
				if _, e := CanonicalIP(ip.String()); e != nil {
					return nil, &CodeError{"IP_LOOKUP_INVALID_CONFIG"}
				}
			}
			var last error
			for _, ip := range ips {
				conn, e := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if e == nil {
					return conn, nil
				}
				last = e
			}
			return nil, last
		}
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return http.ErrUseLastResponse
			}
			_, e := normalizeSourceURL(req.URL.String())
			return e
		}}
		req, e := http.NewRequestWithContext(callCtx, http.MethodGet, raw, nil)
		if e != nil {
			return nil, e
		}
		response, e := client.Do(req)
		if e != nil {
			return nil, &CodeError{"IP_LOOKUP_PROVIDER_UNAVAILABLE"}
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode != http.StatusOK {
			return nil, &CodeError{"IP_LOOKUP_PROVIDER_UNAVAILABLE"}
		}
		data, e := io.ReadAll(io.LimitReader(response.Body, maxProxyBody+1))
		if e != nil || len(data) > maxProxyBody {
			return nil, &CodeError{"IP_LOOKUP_INVALID_RESPONSE"}
		}
		return data, nil
	})
}
