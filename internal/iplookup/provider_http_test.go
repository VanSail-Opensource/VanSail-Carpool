package iplookup

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

type providerSettings struct{ value string }

func (s providerSettings) Optional(context.Context, string) (string, error) { return s.value, nil }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestProviderRequestsEnterQueueAndKeepSecretsOutOfErrors(t *testing.T) {
	t.Parallel()
	for _, id := range IDs {
		t.Run(id, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			queue, err := upstreamqueue.New(upstreamqueue.Config{Name: id, Capacity: 1})
			if err != nil {
				t.Fatal(err)
			}
			if err := queue.Start(ctx); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = queue.Shutdown(context.Background()) }()
			provider := NewHTTPProvider(id, queue, providerSettings{value: "fixture-secret"}).(*httpProvider)
			called := false
			provider.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				called = true
				if r.Method != "GET" || r.URL.Scheme != "https" || r.Header.Get("Accept") != "application/json" {
					t.Fatalf("invalid request: %s %s", r.Method, r.URL.Scheme)
				}
				if id == "abuseipdb" && (r.Header.Get("Key") != "fixture-secret" || r.URL.Query().Get("maxAgeInDays") != "90") {
					t.Error("AbuseIPDB contract mismatch")
				}
				if id == "maxmind" {
					user, password, ok := r.BasicAuth()
					if !ok || user != "123456" || password != "fixture-secret" {
						t.Error("MaxMind auth mismatch")
					}
				}
				if id == "ipqs" && r.URL.Query().Get("strictness") != "3" {
					t.Error("IPQS strictness mismatch")
				}
				if id == "scamalytics" && (r.URL.Host != "api12.scamalytics.com" || r.URL.Path != "/v3/123456/" || r.URL.Query().Get("key") != "fixture-secret") {
					t.Error("Scamalytics v3 contract mismatch")
				}
				return &http.Response{StatusCode: 429, Body: io.NopCloser(strings.NewReader(`{"error":"exhausted"}`)), Header: http.Header{}}, nil
			})}
			_, err = provider.Lookup(ctx, "150.249.241.62", ProviderConfig{ID: id, AccountID: "123456"})
			if !called || ErrorCode(err) != "IP_LOOKUP_PROVIDER_QUOTA" || strings.Contains(err.Error(), "fixture-secret") {
				t.Fatalf("request/error result: called=%v code=%s", called, ErrorCode(err))
			}
			provider.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return nil, errors.New("transport error containing fixture-secret")
			})
			_, err = provider.Lookup(ctx, "150.249.241.62", ProviderConfig{ID: id, AccountID: "123456"})
			if err == nil || strings.Contains(err.Error(), "fixture-secret") {
				t.Fatal("transport error leaked credential")
			}
		})
	}
}
