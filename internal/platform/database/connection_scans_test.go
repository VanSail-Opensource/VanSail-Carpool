package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestConnectionScanReplayAndMonotonicLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 50_002)
	now := time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC)
	input := providerops.ConnectionScanInput{UserID: user.ID, IdempotencyKey: "scan-key",
		RequestFingerprint: "0123456789abcdef", ExpiresAt: now.Add(10 * time.Minute)}
	scan, replayed, err := store.CreateConnectionScan(ctx, input, now)
	if err != nil || replayed || scan.Status != providerops.StatusQueued || scan.ProviderJobID != "" {
		t.Fatalf("CreateConnectionScan() = (%+v, %t, %v)", scan, replayed, err)
	}
	replayedScan, replayed, err := store.CreateOrReplayConnectionScan(ctx, input, now.Add(time.Second))
	if err != nil || !replayed || replayedScan.ID != scan.ID {
		t.Fatalf("CreateOrReplayConnectionScan() = (%+v, %t, %v)", replayedScan, replayed, err)
	}
	conflict := input
	conflict.RequestFingerprint = "fedcba9876543210"
	if _, _, err := store.CreateConnectionScan(ctx, conflict, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflicting replay error = %v, want ErrConflict", err)
	}

	updates := []struct {
		name       string
		update     providerops.ConnectionScanUpdate
		wantStatus providerops.Status
		wantJob    string
		want       float64
		wantErr    error
	}{
		{name: "record request intent", update: providerops.ConnectionScanUpdate{Status: providerops.StatusProcessing, ProgressPercent: 0}, wantStatus: providerops.StatusProcessing},
		{name: "attach provider job", update: providerops.ConnectionScanUpdate{Status: providerops.StatusProcessing, ProviderJobID: "job-1", ProgressPercent: 25}, wantStatus: providerops.StatusProcessing, wantJob: "job-1", want: 25},
		{name: "reject progress regression", update: providerops.ConnectionScanUpdate{Status: providerops.StatusProcessing, ProviderJobID: "job-1", ProgressPercent: 20}, wantErr: ErrConflict},
		{name: "reject job replacement", update: providerops.ConnectionScanUpdate{Status: providerops.StatusProcessing, ProviderJobID: "job-2", ProgressPercent: 30}, wantErr: ErrConflict},
		{name: "complete at one hundred", update: providerops.ConnectionScanUpdate{Status: providerops.StatusSucceeded, ProviderJobID: "job-1", ProgressPercent: 80}, wantStatus: providerops.StatusSucceeded, wantJob: "job-1", want: 100},
	}
	for _, test := range updates {
		t.Run(test.name, func(t *testing.T) {
			got, gotErr := store.UpdateConnectionScan(ctx, scan.ID, test.update, now.Add(time.Minute))
			if !errors.Is(gotErr, test.wantErr) {
				t.Fatalf("UpdateConnectionScan() error = %v, want %v", gotErr, test.wantErr)
			}
			if gotErr == nil && (got.Status != test.wantStatus || got.ProviderJobID != test.wantJob || got.ProgressPercent != test.want) {
				t.Fatalf("UpdateConnectionScan() = %+v", got)
			}
		})
	}
	owned, err := store.ConnectionScanForUser(ctx, scan.ID, user.ID)
	if err != nil || owned.CompletedAt == nil || owned.ProgressPercent != 100 {
		t.Fatalf("ConnectionScanForUser() = (%+v, %v)", owned, err)
	}
	var sensitiveColumns, jobs int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('connection_scans')
		WHERE name IN ('ip','handle','result_json')`).Scan(&sensitiveColumns); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='connection_scan_request'
		AND json_extract(payload,'$.scanId')=?`, scan.ID).Scan(&jobs); err != nil {
		t.Fatal(err)
	}
	if sensitiveColumns != 0 || jobs != 1 {
		t.Fatalf("metadata-only persistence = sensitive columns:%d jobs:%d", sensitiveColumns, jobs)
	}
}

func TestConnectionScanPendingReviewIsTerminal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 50_003)
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	scan, _, err := store.CreateConnectionScan(ctx, providerops.ConnectionScanInput{
		UserID: user.ID, IdempotencyKey: "ambiguous-scan", RequestFingerprint: "0123456789abcdef",
		ExpiresAt: now.Add(10 * time.Minute),
	}, now)
	if err != nil {
		t.Fatalf("CreateConnectionScan() error = %v", err)
	}
	if _, err := store.UpdateConnectionScan(ctx, scan.ID, providerops.ConnectionScanUpdate{
		Status: providerops.StatusProcessing,
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("mark processing: %v", err)
	}
	pending, err := store.UpdateConnectionScan(ctx, scan.ID, providerops.ConnectionScanUpdate{
		Status: providerops.StatusPendingReview, ErrorCode: "CONNECTION_SCAN_START_AMBIGUOUS",
	}, now.Add(2*time.Second))
	if err != nil || pending.CompletedAt == nil || pending.Status != providerops.StatusPendingReview {
		t.Fatalf("mark pending review = (%+v, %v)", pending, err)
	}
	if _, err := store.UpdateConnectionScan(ctx, scan.ID, providerops.ConnectionScanUpdate{
		Status: providerops.StatusProcessing,
	}, now.Add(3*time.Second)); !errors.Is(err, ErrConflict) {
		t.Fatalf("pending-review transition error = %v, want ErrConflict", err)
	}
}
