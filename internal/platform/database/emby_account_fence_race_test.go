package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
)

func TestLegacyEmbyAdmissionWaitsForCommittedOwnerFence(t *testing.T) {
	for _, retry := range []bool{false, true} {
		t.Run(map[bool]string{false: "new setup", true: "provision retry"}[retry], func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			s := newTestStore(t)
			u := createTestUser(t, s, 497693)
			at := time.Now().UTC()
			if _, err := s.AdjustBalance(ctx, u.ID, 1000, "emby-race-seed", "seed", at); err != nil {
				t.Fatal(err)
			}
			input := fencedEmbySetup(u.ID, u.TelegramID)
			if retry {
				if _, _, err := s.QueueEmbySetup(ctx, input, at); err != nil {
					t.Fatal(err)
				}
				if _, err := s.DB().ExecContext(ctx, `DELETE FROM outbox_jobs WHERE kind=$1`, emby.ProvisionOutboxKind); err != nil {
					t.Fatal(err)
				}
			}
			tx, err := s.DB().BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if _, err = tx.ExecContext(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, u.ID); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				if retry {
					_, err := s.RetryEmbyProvisioning(ctx, input.ID, at)
					done <- err
				} else {
					_, _, err := s.QueueEmbySetup(ctx, input, at)
					done <- err
				}
			}()
			deadline := time.Now().Add(5 * time.Second)
			blocked := false
			for time.Now().Before(deadline) {
				if err = s.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock' AND strpos(query,'ORDER BY id FOR SHARE')>0)`).Scan(&blocked); err != nil {
					t.Fatal(err)
				}
				if blocked {
					break
				}
				time.Sleep(5 * time.Millisecond)
			}
			if !blocked {
				t.Fatal("Emby admission did not retain the owner lifecycle share lock")
			}
			if _, err = tx.ExecContext(ctx, `UPDATE users SET account_status='purging',account_generation=1 WHERE id=$1`, u.ID); err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(); err != nil {
				t.Fatal(err)
			}
			if err = <-done; !errors.Is(err, ErrAccountUnavailable) {
				t.Fatal("late setup/retry crossed committed cleanup", err)
			}
			var queued int
			if err = s.DB().QueryRowContext(ctx, `SELECT count(*) FROM outbox_jobs WHERE kind=$1`, emby.ProvisionOutboxKind).Scan(&queued); err != nil || queued != 0 {
				t.Fatal("late Emby job appeared after closure", queued, err)
			}
			want := "1000"
			if retry {
				want = "725"
			}
			assertBalance(t, s, u.ID, want)
		})
	}
}
