package xnode

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

type zoneTransport func(*http.Request) (*http.Response, error)

func (f zoneTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func zoneQueue(t *testing.T) *upstreamqueue.Queue {
	t.Helper()
	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "zone-fixture", Capacity: 4})
	if err != nil {
		t.Fatal(err)
	}
	if err = queue.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := queue.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return queue
}

func TestCloudflareZonePagination(t *testing.T) {
	queue := zoneQueue(t)
	calls := 0
	client := &http.Client{Transport: zoneTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "api.cloudflare.com" || r.URL.Query().Get("per_page") != "50" || r.URL.Query().Get("page") != fmt.Sprint(calls) || r.Header.Get("Authorization") != "Bearer secret" {
			t.Error("invalid queued Cloudflare request")
			return nil, errors.New("invalid request")
		}
		body := fmt.Sprintf(`{"success":true,"result":[{"id":"zone-%d","name":"zone-%d.example","credentials":"never projected"}],"result_info":{"page":%d,"total_pages":2}}`, calls, calls, calls)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	items, err := listCloudflareZones(context.Background(), queue, client, "secret")
	if err != nil || calls != 2 || len(items) != 2 || items[1].Name != "zone-2.example" {
		t.Fatalf("pagination: %#v, %v, calls=%d", items, err, calls)
	}
}

func TestCloudflareZoneErrorsAndCancellation(t *testing.T) {
	for _, test := range []struct {
		name, body string
		status     int
	}{
		{"denied", `{"errors":[{"message":"secret"}]}`, 403},
		{"limited", `secret`, 429},
		{"provider-error", `{"success":false,"result":[]}`, 200},
		{"malformed", `secret`, 200},
		{"oversized", strings.Repeat("x", (1<<20)+1), 200},
		{"missing-results", `{"success":true}`, 200},
		{"wrong-page", `{"success":true,"result":[],"result_info":{"page":3}}`, 200},
		{"truncated", `{"success":true,"result":[],"result_info":{"total_pages":2}}`, 200},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Transport: zoneTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(test.body))}, nil
			})}
			items, err := listCloudflareZones(context.Background(), zoneQueue(t), client, "secret")
			if !errors.Is(err, ErrUnavailable) || items != nil || strings.Contains(err.Error(), "secret") {
				t.Fatalf("unsafe/partial failure: %v, %v", items, err)
			}
		})
	}
	t.Run("cancelled-before-admission", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		client := &http.Client{Transport: zoneTransport(func(*http.Request) (*http.Response, error) {
			t.Error("cancelled request must not execute")
			return nil, errors.New("unreachable")
		})}
		_, err := listCloudflareZones(ctx, zoneQueue(t), client, "secret")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation lost: %v", err)
		}
	})
	t.Run("empty", func(t *testing.T) {
		client := &http.Client{Transport: zoneTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"success":true,"result":[]}`))}, nil
		})}
		items, err := listCloudflareZones(context.Background(), zoneQueue(t), client, "secret")
		if err != nil || items == nil || len(items) != 0 {
			t.Fatalf("empty zones: %v, %v", items, err)
		}
	})
}
