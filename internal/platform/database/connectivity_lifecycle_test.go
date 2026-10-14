package database

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
)

func TestConnectivityAttemptLifecycleSealsEvidenceAndNullableFields(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 10, 9, 12, 0, 0, 123, time.UTC)
	probe := connectivityTestAttempt(1, connectivityTestHost, now)
	setup := connectivityTestAttempt(2, "", now.Add(time.Second))
	seedConnectivityAttempt(t, store, probe)
	seedConnectivityAttempt(t, store, setup)
	page, err := store.ConnectivityHistory(ctx, "", "", 50, now.Add(time.Minute))
	if err != nil || len(page.Items) != 2 {
		t.Fatalf("running history=%+v, err=%v", page, err)
	}
	for _, item := range page.Items {
		if item.Status != "running" || item.FinishedAt != nil || item.LatencyMS != nil || item.HTTPStatus != nil {
			t.Fatalf("unexpected initial attempt=%+v", item)
		}
	}
	if err := store.BeginConnectivityAttempt(ctx, probe); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate begin=%v", err)
	}
	if err := store.FinishConnectivityAttempt(ctx, probe.ID, connectivityOutcome(), now.Add(-time.Nanosecond)); !errors.Is(err, ErrConflict) {
		t.Fatalf("finish before start=%v", err)
	}
	if err := store.FinishConnectivityAttempt(ctx, probe.ID, connectivityOutcome(), now.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := store.FinishConnectivityAttempt(ctx, probe.ID, connectivity.Outcome{Status: "error"}, now.Add(6*time.Second)); !errors.Is(err, ErrConflict) {
		t.Fatalf("terminal overwrite=%v", err)
	}
	if err := store.FinishConnectivityAttempt(ctx, setup.ID, connectivity.Outcome{Status: "error", ErrorCode: "CONNECTIVITY_SUBSCRIPTION_FAILED"}, now.Add(6*time.Second)); err != nil {
		t.Fatal(err)
	}
	page, err = store.ConnectivityHistory(ctx, "", "", 50, now.Add(time.Minute))
	if err != nil || len(page.Items) != 2 {
		t.Fatalf("terminal history=%+v, err=%v", page, err)
	}
	if got := page.Items[0]; got.HostUUID != "" || got.FinishedAt == nil || got.Status != "error" || got.LatencyMS != nil || got.HTTPStatus != nil {
		t.Fatalf("setup error=%+v", got)
	}
	if got := page.Items[1]; got.Status != "connected" || got.LatencyMS == nil || *got.LatencyMS != 4.25 || got.HTTPStatus == nil || *got.HTTPStatus != 204 {
		t.Fatalf("probe result=%+v", got)
	}
}

func TestConnectivityRejectsInvalidOrUnsanitizedOutcomes(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	negative, infinite, nan, invalidHTTP := -1.0, math.Inf(1), math.NaN(), 600
	tests := []struct {
		name    string
		outcome connectivity.Outcome
	}{
		{"running", connectivity.Outcome{Status: "running"}},
		{"unknown", connectivity.Outcome{Status: "success"}},
		{"raw error", connectivity.Outcome{Status: "failed", ErrorCode: "secret@proxy.example:443"}},
		{"negative latency", connectivity.Outcome{Status: "failed", LatencyMS: &negative}},
		{"infinite latency", connectivity.Outcome{Status: "failed", LatencyMS: &infinite}},
		{"nan latency", connectivity.Outcome{Status: "failed", LatencyMS: &nan}},
		{"invalid HTTP", connectivity.Outcome{Status: "failed", HTTPStatus: &invalidHTTP}},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			attempt := connectivityTestAttempt(index+1, connectivityTestHost, now)
			seedConnectivityAttempt(t, store, attempt)
			if err := store.FinishConnectivityAttempt(context.Background(), attempt.ID, test.outcome, now.Add(time.Second)); !errors.Is(err, ErrConflict) {
				t.Fatalf("invalid outcome=%v", err)
			}
		})
	}
}

func TestConnectivityCancelledContextDoesNotInsert(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	attempt := connectivityTestAttempt(1, connectivityTestHost, time.Now())
	if err := store.BeginConnectivityAttempt(ctx, attempt); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled begin=%v", err)
	}
	page, err := store.ConnectivityHistory(context.Background(), "", "", 50, time.Now())
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("cancelled attempt persisted=%+v,%v", page, err)
	}
}
