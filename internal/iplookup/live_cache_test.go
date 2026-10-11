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

func TestLivePersistentCacheDoesNotExpireOrInvalidateAndRejectsFreeRefresh(t *testing.T) {
	s, scores, _ := integratedLiveService(t, false)
	s.repository = &snapshotMemory{}
	now := time.Now()
	s.now = func() time.Time { return now }
	for range 2 {
		if _, err := s.Details(context.Background(), "member", "1.1.1.1"); err != nil {
			t.Fatal(err)
		}
	}
	if scores.Load() != 1 {
		t.Fatal("cache hit contacted upstream")
	}
	if _, err := s.DetailsWithOptions(context.Background(), "member", "1.1.1.1", LiveOptions{Refresh: true}); ErrorCode(err) != "IP_LOOKUP_REFRESH_REQUIRED" {
		t.Fatal("free refresh accepted")
	}
	c, err := DecodeConfig(s.settings.(liveConfigSettings)[SettingKey])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RefreshRun(context.Background(), "1.1.1.1", c, "paid-run", nil); err != nil {
		t.Fatal(err)
	}
	s.settings.(liveConfigSettings)[CredentialKey("ipapi")] = "replacement"
	now = now.Add(24 * time.Hour)
	if _, err := s.Details(context.Background(), "member", "1.1.1.1"); err != nil {
		t.Fatal(err)
	}
	if scores.Load() != 2 {
		t.Fatal("cache expired or was invalidated")
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

func TestLiveAttemptedOutageCachedAndPreAttemptFailureExcluded(t *testing.T) {
	s, scores, _ := integratedLiveService(t, false)
	snapshots := &snapshotMemory{}
	s.repository = snapshots
	original := s.client.Transport
	s.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "internetdb.shodan.io" {
			return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader(`{}`)), Header: http.Header{}}, nil
		}
		return original.RoundTrip(r)
	})
	for range 2 {
		if _, err := s.Details(context.Background(), "a", "1.1.1.1"); err != nil {
			t.Fatal(err)
		}
	}
	if scores.Load() != 1 || len(snapshots.rows) != 1 {
		t.Fatal("attempted outage was not cached")
	}
	s.queues = nil
	if _, err := s.Details(context.Background(), "a", "8.8.8.8"); err != nil {
		t.Fatal(err)
	}
	if len(snapshots.rows) != 1 {
		t.Fatal("pre-attempt failure cached")
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
