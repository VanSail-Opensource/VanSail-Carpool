package httpapi

import (
	"context"
	"net/http"
	"net/netip"
	"strings"
)

type clientIPKey struct{}

// RemoteAddr remains the transport peer for native identity and access logs.
// Only authentication receives a proxy-derived, canonical client address.
func clientIPMiddleware(prefixes []netip.Prefix) func(http.Handler) http.Handler {
	trusted := append([]netip.Prefix(nil), prefixes...)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			address := resolveClientIP(r, trusted)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), clientIPKey{}, address)))
		})
	}
}

func requestClientIP(r *http.Request) string {
	if address, ok := r.Context().Value(clientIPKey{}).(netip.Addr); ok && address.IsValid() {
		return address.String()
	}
	if address := peerAddress(r.RemoteAddr); address.IsValid() {
		return address.String()
	}
	return "unknown"
}

func peerAddress(raw string) netip.Addr {
	if endpoint, err := netip.ParseAddrPort(strings.TrimSpace(raw)); err == nil {
		return endpoint.Addr().Unmap()
	}
	return headerAddress(raw)
}

func headerAddress(raw string) netip.Addr {
	address, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil || address.Zone() != "" {
		return netip.Addr{}
	}
	return address.Unmap()
}

func trustedProxy(address netip.Addr, prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func resolveClientIP(r *http.Request, prefixes []netip.Prefix) netip.Addr {
	peer := peerAddress(r.RemoteAddr)
	if !peer.IsValid() || !trustedProxy(peer, prefixes) {
		return peer
	}
	forwarded := r.Header.Values("X-Forwarded-For")
	if len(forwarded) == 0 {
		values := r.Header.Values("X-Real-IP")
		if len(values) == 1 {
			if address := headerAddress(values[0]); address.IsValid() {
				return address
			}
		}
		return peer
	}
	// Traverse only the trusted suffix. Arbitrary client-supplied left prefixes
	// cannot choose a victim IP or collapse unrelated visitors into the proxy bucket.
	client, hops := peer, 0
	for i := len(forwarded) - 1; i >= 0; i-- {
		rest := forwarded[i]
		for {
			if hops >= 32 {
				return peer
			}
			hops++
			comma := strings.LastIndexByte(rest, ',')
			token := rest
			if comma >= 0 {
				token = rest[comma+1:]
			}
			address := headerAddress(token)
			if !address.IsValid() {
				return peer
			}
			client = address
			if !trustedProxy(address, prefixes) {
				return client
			}
			if comma < 0 {
				break
			}
			rest = rest[:comma]
		}
	}
	return client
}
