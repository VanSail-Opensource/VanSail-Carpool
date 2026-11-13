package iplookup

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestASNPrefixFallbackDistinguishesNullFromObservedZero(t *testing.T) {
	for _, tc := range []struct {
		name, prefixes string
		known          bool
		count          int
	}{
		{"null", `null`, false, 0},
		{"wrong type", `{}`, false, 0},
		{"empty", `[]`, true, 0},
		{"announced", `["1.1.1.0/24"]`, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, _ := integratedLiveService(t, false)
			original := s.client.Transport
			s.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				body := ""
				switch {
				case strings.Contains(r.URL.Path, "routing-status"):
					body = `{"data":{"announced_space":{}}}`
				case strings.HasPrefix(r.URL.Path, "/asn/"):
					body = tc.prefixes
					if tc.count > 0 && r.URL.Query().Get("af") == "6" {
						body = `["2606:4700:4700::/48"]`
					}
				default:
					return original.RoundTrip(r)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
			})
			result := s.asn(context.Background(), 13335, Config{})
			if (result.IPv4Prefixes != nil) != tc.known || (result.IPv6Prefixes != nil) != tc.known || (result.Announced != nil) != tc.known {
				t.Fatalf("unknown response became a zero observation: %+v", result)
			}
			if tc.known && (*result.IPv4Prefixes != tc.count || *result.IPv6Prefixes != tc.count || *result.Announced != (tc.count > 0)) {
				t.Fatalf("valid prefix observation lost: %+v", result)
			}
			if !tc.known && (result.Sources["ipv4Prefixes"].Status == "success" || result.Sources["ipv6Prefixes"].Status == "success") {
				t.Fatal("invalid fallback marked successful")
			}
		})
	}
}
