package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestConnectionScanUpdateRechecksConcurrentProgressAndCompletion(t *testing.T) {
	for _, winner := range []struct {
		status   providerops.Status
		progress float64
	}{{providerops.StatusProcessing, 80}, {providerops.StatusSucceeded, 100}} {
		t.Run(string(winner.status), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			store := newTestStore(t)
			user := createTestUser(t, store, 900_607)
			now := time.Now().UTC()
			scan, _, err := store.CreateConnectionScan(ctx, providerops.ConnectionScanInput{UserID: user.ID,
				IdempotencyKey: "concurrent-progress", RequestFingerprint: "concurrent-progress-proof", ExpiresAt: now.Add(time.Hour)}, now)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = store.StartConnectionScan(ctx, scan.ID, now); err != nil {
				t.Fatal(err)
			}
			blocker, err := store.DB().BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer blocker.Rollback()
			if _, err = blocker.ExecContext(ctx, `UPDATE connection_scans SET status=$1,progress_percent=$2,provider_job_id='provider-job',
			 completed_at=CASE WHEN $1='succeeded' THEN $3 ELSE NULL END,updated_at=$3 WHERE id=$4`, winner.status, winner.progress, stamp(now.Add(time.Second)), scan.ID); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				_, err := store.UpdateConnectionScan(ctx, scan.ID, providerops.ConnectionScanUpdate{
					Status: providerops.StatusProcessing, ProviderJobID: "provider-job", ProgressPercent: 20}, now.Add(2*time.Second))
				done <- err
			}()
			ticker := time.NewTicker(time.Millisecond)
			defer ticker.Stop()
			for {
				var waiting bool
				if err := store.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity
				 WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock'
				 AND strpos(query,'connection_scans')>0)`).Scan(&waiting); err != nil {
					t.Fatal(err)
				}
				if waiting {
					break
				}
				select {
				case err := <-done:
					t.Fatal("competing update did not wait for the row", err)
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				case <-ticker.C:
				}
			}
			if err := blocker.Commit(); err != nil {
				t.Fatal(err)
			}
			if err := <-done; !errors.Is(err, ErrConflict) {
				t.Fatal("stale poll overwrote a newer committed result", err)
			}
			current, err := store.ConnectionScanByID(ctx, scan.ID)
			if err != nil || current.Status != winner.status || current.ProgressPercent != winner.progress || current.ProviderJobID != "provider-job" ||
				(current.CompletedAt != nil) != (winner.status == providerops.StatusSucceeded) {
				t.Fatal("concurrent result did not survive", current, err)
			}
		})
	}
}
