package iplookup

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestLiveCompletionCacheExpiryRefreshAndCredentials(t *testing.T) {
	s, scores, _ := integratedLiveService(t, false)
	now := time.Now()
	s.now = func() time.Time { return now }
	load := func(refresh bool) {
		t.Helper()
		if _, err := s.DetailsWithOptions(context.Background(), "member", "1.1.1.1", LiveOptions{Refresh: refresh}); err != nil {
			t.Fatal(err)
		}
	}
	load(false)
	load(false)
	if scores.Load() != 1 {
		t.Fatal("cache hit contacted upstream")
	}
	load(true)
	if scores.Load() != 2 {
		t.Fatal("refresh reused completed cache")
	}
	s.settings.(liveConfigSettings)[CredentialKey("ipapi")] = "replacement"
	load(false)
	if scores.Load() != 3 {
		t.Fatal("credential edit reused prior completion")
	}
	now = now.Add(time.Minute)
	load(false)
	if scores.Load() != 4 {
		t.Fatal("expired completion reused")
	}
}

func TestLiveStreamPublishesTopologyBeforeSlowSourceAndCompletes(t *testing.T) {
	s, _, _ := integratedLiveService(t, false)
	original := s.client.Transport
	release := make(chan struct{})
	s.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "internetdb.shodan.io" {
			select {
			case <-release:
			case <-r.Context().Done():
				return nil, r.Context().Err()
			}
		}
		return original.RoundTrip(r)
	})
	finished := make(chan error, 1)
	topology := make(chan struct{}, 1)
	var last int
	go func() {
		_, err := s.DetailsWithOptions(context.Background(), "a", "1.1.1.1", LiveOptions{Emit: func(event LiveEvent) error {
			if event.Type == "section" && event.Section == "topology" {
				select {
				case topology <- struct{}{}:
				default:
				}
			}
			if event.Progress != nil {
				if event.Progress.Completed < last {
					return &CodeError{"regressed"}
				}
				last = event.Progress.Completed
			}
			return nil
		}})
		finished <- err
	}()
	select {
	case <-topology:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("topology waited for slow host source")
	}
	select {
	case <-finished:
		t.Fatal("completed before slow source")
	default:
	}
	close(release)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	if last != 8 {
		t.Fatal("stream never completed")
	}
}

func TestLiveOutageNotCachedAndByteBound(t *testing.T) {
	s, scores, _ := integratedLiveService(t, false)
	original := s.client.Transport
	s.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "internetdb.shodan.io" {
			return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader(`{}`)), Header: http.Header{}}, nil
		}
		return original.RoundTrip(r)
	})
	for i := 0; i < 2; i++ {
		if _, err := s.Details(context.Background(), "a", "1.1.1.1"); err != nil {
			t.Fatal(err)
		}
	}
	if scores.Load() != 2 || len(s.cache) != 0 {
		t.Fatal("outage was cached")
	}
	s.mu.Lock()
	for i := 0; i < 70; i++ {
		s.storeLiveCache(asnText(uint32(i)), make([]byte, 300000))
	}
	s.mu.Unlock()
	bytes := 0
	for _, entry := range s.cache {
		bytes += len(entry.data)
	}
	if len(s.cache) > 64 || bytes > 16*1024*1024 {
		t.Fatal("unbounded live cache")
	}
}

func TestCheckProgressDoesNotExposeCheckpointAndRespectsSkippedStages(t *testing.T) {
	r := Report{Status: "processing", Checkpoint: &Checkpoint{Stages: []Stage{{ID: "a", Status: "success"}, {ID: "b", Status: "processing"}, {ID: "c", Status: "disabled"}, {ID: "d", Status: "skipped"}}}}
	p := CheckProgress(r)
	if p.Total != 3 || p.Completed != 2 || p.CurrentProvider != "b" {
		t.Fatalf("%+v", p)
	}
	data, err := json.Marshal(Check{Progress: p})
	if err != nil || strings.Contains(string(data), "checkpoint") {
		t.Fatal("private checkpoint leaked")
	}
}
