package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestConnectionScanAdmissionRetainsReplayButRejectsClosedOwner(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 900_603)
	at := time.Now().UTC()
	input := providerops.ConnectionScanInput{ActorUserID: scanAdministrator(t, store).ID, UserID: user.ID, IdempotencyKey: "admitted", RequestFingerprint: "admitted-diagnostics-proof", ExpiresAt: at.Add(time.Minute)}
	first, _, err := store.CreateConnectionScan(ctx, input, at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET account_status='purging' WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	if replay, reused, err := store.CreateConnectionScan(ctx, input, at); err != nil || !reused || replay.ID != first.ID {
		t.Fatal("accepted scan lost read-only replay", reused, err)
	}
	input.IdempotencyKey = "new-after-closure"
	if _, _, err := store.CreateConnectionScan(ctx, input, at); !errors.Is(err, ErrAccountUnavailable) {
		t.Fatal("closed owner admitted a new scan", err)
	}
	var scans, jobs int
	if err := store.DB().QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM connection_scans WHERE user_id=$1),
		(SELECT COUNT(*) FROM outbox_jobs WHERE kind='connection_scan_request')`, user.ID).Scan(&scans, &jobs); err != nil || scans != 1 || jobs != 1 {
		t.Fatal("rejected scan left durable work", scans, jobs, err)
	}
}

func TestConnectionScanStartFencesExpiryAndCurrentAccount(t *testing.T) {
	for _, test := range []struct {
		name, state, code string
		remaining         time.Duration
	}{
		{"live request", "active", "", time.Minute},
		{"exact expiry", "active", "CONNECTION_SCAN_EXPIRED", 0},
		{"cleanup already started", "purging", "CONNECTION_SCAN_ACCOUNT_UNAVAILABLE", time.Minute},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			user := createTestUser(t, store, 900_604)
			at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
			created, _, err := store.CreateConnectionScan(ctx, providerops.ConnectionScanInput{ActorUserID: scanAdministrator(t, store).ID, UserID: user.ID,
				IdempotencyKey: "delayed-start", RequestFingerprint: "delayed-diagnostics-proof", ExpiresAt: at.Add(test.remaining)}, at.Add(-time.Hour))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE users SET account_status=$1 WHERE id=$2`, test.state, user.ID); err != nil {
				t.Fatal(err)
			}
			started, err := store.StartConnectionScan(ctx, created.ID, at)
			if err != nil || started.ErrorCode != test.code || started.ProviderJobID != "" {
				t.Fatal("incorrect start outcome", started, err)
			}
			if test.code == "" && (started.Status != providerops.StatusProcessing || started.CompletedAt != nil) ||
				test.code != "" && (started.Status != providerops.StatusFailed || started.CompletedAt == nil || started.CompletedAt.Before(at)) {
				t.Fatal("incorrect admission lifecycle", started)
			}
		})
	}
}

func TestConnectionScanStartChargesOwnerLockWaitingTime(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	store := newTestStore(t)
	user := createTestUser(t, store, 900_605)
	at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	created, _, err := store.CreateConnectionScan(ctx, providerops.ConnectionScanInput{ActorUserID: scanAdministrator(t, store).ID, UserID: user.ID,
		IdempotencyKey: "lock-delayed-start", RequestFingerprint: "lock-delay-proof", ExpiresAt: at.Add(50 * time.Millisecond)}, at.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	blocker, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback()
	if _, err = blocker.ExecContext(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, user.ID); err != nil {
		t.Fatal(err)
	}
	type result struct {
		scan providerops.ConnectionScan
		err  error
	}
	done := make(chan result, 1)
	go func() {
		scan, err := store.StartConnectionScan(ctx, created.ID, at)
		done <- result{scan, err}
	}()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		if err := store.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity
			WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock'
			AND strpos(query,'SELECT account_status FROM users WHERE id=$1 FOR SHARE')>0)`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case value := <-done:
			t.Fatal("start did not wait for owner", value)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
		}
	}
	// The worker clock was captured before the lock wait. Its accepted start
	// must still expire while blocked, even when the logical date is fixed.
	select {
	case <-time.After(60 * time.Millisecond):
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err := blocker.Commit(); err != nil {
		t.Fatal(err)
	}
	value := <-done
	if value.err != nil || value.scan.Status != providerops.StatusFailed || value.scan.ErrorCode != "CONNECTION_SCAN_EXPIRED" ||
		value.scan.CompletedAt == nil || value.scan.CompletedAt.Before(created.ExpiresAt) {
		t.Fatal("expired request admitted after lock wait", value)
	}
}
