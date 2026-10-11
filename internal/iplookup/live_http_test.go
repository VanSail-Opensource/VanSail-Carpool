package iplookup

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/txyyddss/Remna-User-Panel/internal/platform/upstreamqueue"
)

func liveTestService(t *testing.T, source string, reply func(*http.Request) (int, string)) *LiveService {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	q, err := upstreamqueue.New(upstreamqueue.Config{Name: source, Capacity: 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q.Shutdown(context.Background()) })
	s := NewLiveService(providerSettings{value: "fixture-secret"}, map[string]*upstreamqueue.Queue{source: q})
	s.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		status, body := reply(r)
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
	return s
}

func TestLiveHTTPRequiresQueueAndRedactsFailure(t *testing.T) {
	var called atomic.Bool
	s := liveTestService(t, "abuseipdb", func(r *http.Request) (int, string) {
		called.Store(true)
		if r.Header.Get("Key") != "fixture-secret" {
			t.Error("missing auth")
		}
		return 402, `{"error":"fixture-secret"}`
	})
	c := Config{Providers: []ProviderConfig{{ID: "abuseipdb", Enabled: true}}}
	r := s.block(context.Background(), "1.1.1.1", c)
	if !called.Load() || r.Status != "restricted" || r.ReportedAddresses != nil || r.AddressCapacity != "256" {
		t.Fatalf("%+v", r)
	}
	s.queues = map[string]*upstreamqueue.Queue{}
	called.Store(false)
	_, err := s.liveHTTP(context.Background(), "abuseipdb", "/api/v2/check-block", nil, nil, "secret")
	if called.Load() || ErrorCode(err) != "IP_DETAILS_UNAVAILABLE" {
		t.Fatal("bypassed admission queue")
	}
}

func TestLiveBlockCountsDistinctAndRejectsUnknown(t *testing.T) {
	for _, tc := range []struct {
		rows  string
		known bool
		count int
	}{{`[]`, true, 0}, {`[{"ipAddress":"1.1.1.1"},{"ipAddress":"1.1.1.1"}]`, true, 1}, {`null`, false, 0}, {`[{"ipAddress":"8.8.8.8"}]`, false, 0}} {
		s := liveTestService(t, "abuseipdb", func(*http.Request) (int, string) {
			return 200, `{"data":{"networkAddress":"1.1.1.0","netmask":"255.255.255.0","reportedAddress":` + tc.rows + `}}`
		})
		r := s.block(context.Background(), "1.1.1.1", Config{Providers: []ProviderConfig{{ID: "abuseipdb", Enabled: true}}})
		if (r.ReportedAddresses != nil) != tc.known || tc.known && *r.ReportedAddresses != tc.count {
			t.Fatalf("%s: %+v", tc.rows, r)
		}
	}
}

func TestLiveCloudflareKeepsOtherAndWindow(t *testing.T) {
	s := liveTestService(t, CloudflareID, func(r *http.Request) (int, string) {
		if r.URL.Query().Get("asn") != "13335" || r.URL.Query().Get("dateRange") != "7d" || r.Header.Get("Authorization") != "Bearer fixture-secret" {
			t.Error("Cloudflare request mismatch")
		}
		return 200, `{"success":true,"result":{"summary_0":{"desktop":"43.2","mobile":"55.7","other":"1.1"},"meta":{"dateRange":[{"startTime":"2026-10-01","endTime":"2026-10-08"}],"lastUpdated":"2026-10-08"}}}`
	})
	r := s.trafficSummary(context.Background(), 13335, "DEVICE_TYPE", "fixture-secret")
	if r.Status != "success" || r.Values["other"] != 1.1 || r.StartTime != "2026-10-01" || r.EndTime != "2026-10-08" {
		t.Fatalf("%+v", r)
	}
}
