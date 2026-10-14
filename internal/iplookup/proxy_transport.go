package iplookup

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

func (s *EnhancedService) downloadInventory(ctx context.Context, raw string, c Config, emit func(string)) error {
	var gate sync.Mutex
	started, stopped := false, false
	done := make(chan struct{})
	_, err := upstreamqueue.Do(ctx, s.queues["proxy_lists"], func(ctx context.Context) (bool, error) {
		gate.Lock()
		if stopped {
			gate.Unlock()
			return false, context.Canceled
		}
		started = true
		gate.Unlock()
		defer close(done)
		callCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		transport.ResponseHeaderTimeout = 30 * time.Second
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
			return false, e
		}
		response, e := client.Do(req)
		if e != nil {
			return false, &CodeError{"IP_PROXY_DOWNLOAD"}
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode != http.StatusOK {
			return false, &CodeError{"IP_PROXY_DOWNLOAD"}
		}
		return true, readInventory(callCtx, response.Body, emit)
	})
	// Cancellation can return from Do before its callback settles.
	gate.Lock()
	stopped = true
	join := started
	gate.Unlock()
	if join {
		<-done
	}
	return err
}
