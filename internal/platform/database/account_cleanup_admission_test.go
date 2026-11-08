package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestCleanupOldBlockedPurgeHasNoInferredGraceOrDestructiveAdmission(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497661)
	at := time.Now().UTC().Truncate(time.Microsecond)
	if err := s.ConfigureAccountCleanupAdmins(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetAccountCleanupBot(ctx, 900001, at); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE users SET account_status='purging',account_generation=1,bot_blocked=1,last_purged_at=$2 WHERE id=$1`, u.ID, stamp(at.Add(-30*24*time.Hour))); err != nil {
		t.Fatal(err)
	}
	if err := s.EnqueueAccountPurges(ctx, at); err != nil {
		t.Fatal(err)
	}
	var jobs int
	if err := s.DB().QueryRowContext(ctx, `SELECT count(*) FROM outbox_jobs WHERE kind='account_purge'`).Scan(&jobs); err != nil || jobs != 0 {
		t.Fatal("old flag inferred proof", jobs, err)
	}
	if _, err := s.AccountPurge(ctx, u.ID, 1); !errors.Is(err, ErrCleanupAdmissionRequired) {
		t.Fatal(err)
	}
	if err := s.CompleteAccountPurge(ctx, u.ID, 1, at); !errors.Is(err, ErrCleanupAdmissionRequired) {
		t.Fatal(err)
	}
	if reason, err := s.AccountCleanupAdmissionReason(ctx, u.ID, 1, at); err != nil || reason != "seven_day_evidence_required" {
		t.Fatal(reason, err)
	}
}

func TestCleanupAdministratorPromotionProtectsProbeAndFinalTransaction(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497662)
	at := time.Now().UTC().Truncate(time.Microsecond)
	p := cleanupTestProbe(t, s, u, at, false)
	if _, err := s.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, u.ID); err != nil {
		t.Fatal(err)
	}
	if r := cleanupTestComplete(t, s, p, model.AccountCleanupObservation{SendAction: "blocked", GetChat: "private"}, at); r.Accepted || r.Admitted {
		t.Fatal(r)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE users SET role='user' WHERE id=$1`, u.ID); err != nil {
		t.Fatal(err)
	}
	admitTestAccountCleanup(t, s, u, at)
	if _, err := s.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, u.ID); err != nil {
		t.Fatal(err)
	}
	if valid, err := s.AccountCleanupAdmissionValid(ctx, u.ID, 1, at); err != nil || valid {
		t.Fatal(valid, err)
	}
	if err := s.CompleteAccountPurge(ctx, u.ID, 1, at); err != nil {
		t.Fatal(err)
	}
	got, err := s.UserByID(ctx, u.ID)
	if err != nil || got.AccountStatus != "purging" || got.Role != "admin" {
		t.Fatal("administrator finalized", got, err)
	}
}

func TestCleanupConfiguredProtectionAndUninitializedProtectionFailClosed(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497663)
	at := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := s.SetAccountCleanupBot(ctx, 900001, at); err != nil {
		t.Fatal(err)
	}
	if err := s.EnqueueAccountCleanupChecks(ctx, at, 64); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.DB().QueryRowContext(ctx, `SELECT count(*) FROM account_cleanup_checks`).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	p := cleanupTestProbe(t, s, u, at, false)
	if err := s.ConfigureAccountCleanupAdmins([]int64{u.TelegramID}); err != nil {
		t.Fatal(err)
	}
	if r := cleanupTestComplete(t, s, p, model.AccountCleanupObservation{SendAction: "blocked", GetChat: "private"}, at); r.Accepted || r.Admitted {
		t.Fatal(r)
	}
	if err := s.HandleTelegramBotStatus(ctx, u.TelegramID, 1, 1, true, false, at); err != nil {
		t.Fatal(err)
	}
	got, err := s.UserByID(ctx, u.ID)
	if err != nil || got.BotBlocked || got.AccountGeneration != 0 {
		t.Fatal(got, err)
	}
}

func TestCleanupAdmissionCannotExtendExpiredFinalWithBackdatedQueueTime(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497664)
	old := time.Now().UTC().Truncate(time.Microsecond).Add(-6 * time.Minute)
	admitTestAccountCleanup(t, s, u, old)
	if valid, err := s.AccountCleanupAdmissionValid(ctx, u.ID, 1, old); err != nil || valid {
		t.Fatal("queue timestamp extended final evidence", valid, err)
	}
}

func TestCleanupEvidenceCannotBeRewrittenOrDeleted(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497665)
	at := time.Now().UTC().Truncate(time.Microsecond)
	p := cleanupTestProbe(t, s, u, at, false)
	cleanupTestComplete(t, s, p, model.AccountCleanupObservation{SendAction: "blocked", GetChat: "private"}, at)
	for _, statement := range []string{`UPDATE account_cleanup_evidence SET qualifying=0 WHERE nonce=$1`, `DELETE FROM account_cleanup_evidence WHERE nonce=$1`} {
		if _, err := s.DB().ExecContext(ctx, statement, p.Nonce); err == nil {
			t.Fatal("cleanup evidence was mutable")
		}
	}
}
