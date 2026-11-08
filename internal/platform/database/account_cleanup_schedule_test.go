package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestCleanupSchedulePersistsNonceAndCoalescesAgedPendingJob(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497651)
	at := time.Now().UTC().Truncate(time.Microsecond)
	old := cleanupTestProbe(t, s, u, at.Add(-3*time.Minute), false)
	if _, err := s.SetAccountCleanupBot(ctx, old.BotID, at); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"userId": u.ID, "generation": 0, "nonce": old.Nonce})
	if err := insertOutbox(ctx, s, "account_cleanup_probe", string(body), at.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	var original string
	if err := s.DB().QueryRowContext(ctx, `SELECT id FROM outbox_jobs WHERE kind='account_cleanup_probe'`).Scan(&original); err != nil {
		t.Fatal(err)
	}
	if err := s.EnqueueAccountCleanupChecks(ctx, at, 64); err != nil {
		t.Fatal(err)
	}
	var id, payload string
	var count int
	if err := s.DB().QueryRowContext(ctx, `SELECT id,payload FROM outbox_jobs WHERE kind='account_cleanup_probe'`).Scan(&id, &payload); err != nil || id != original {
		t.Fatal(id, err)
	}
	var input struct {
		UserID     string `json:"userId"`
		Generation int64  `json:"generation"`
		Nonce      string `json:"nonce"`
	}
	if err := json.Unmarshal([]byte(payload), &input); err != nil || input.Nonce == old.Nonce {
		t.Fatal(input, err)
	}
	probe, err := s.AccountCleanupProbe(ctx, u.ID, 0, input.Nonce)
	if err != nil || probe == nil || probe.Nonce != input.Nonce {
		t.Fatal("queue exposed non-durable nonce", probe, err)
	}
	if err = s.EnqueueAccountCleanupChecks(ctx, at.Add(time.Second), 64); err != nil {
		t.Fatal(err)
	}
	if err = s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='account_cleanup_probe'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate pending probes", count, err)
	}
}

func TestCleanupSchedulingDoesNotReplaceProcessingNonce(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497652)
	at := time.Now().UTC().Truncate(time.Microsecond)
	p := cleanupTestProbe(t, s, u, at.Add(-3*time.Minute), false)
	if _, err := s.SetAccountCleanupBot(ctx, p.BotID, at); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"userId": u.ID, "generation": 0, "nonce": p.Nonce})
	if err := insertOutbox(ctx, s, "account_cleanup_probe", string(body), at.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE outbox_jobs SET status='processing' WHERE kind='account_cleanup_probe'`); err != nil {
		t.Fatal(err)
	}
	if err := s.EnqueueAccountCleanupChecks(ctx, at, 64); err != nil {
		t.Fatal(err)
	}
	var nonce string
	if err := s.DB().QueryRowContext(ctx, `SELECT pending_nonce FROM account_cleanup_checks WHERE user_id=$1`, u.ID).Scan(&nonce); err != nil || nonce != p.Nonce {
		t.Fatal("processing work retargeted", nonce, err)
	}
}

func TestCleanupFinalRefreshWakesSameCanonicalPurgeJob(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497653)
	at := time.Now().UTC().Truncate(time.Microsecond)
	admitTestAccountCleanup(t, s, u, at.Add(-6*time.Minute))
	var id string
	if err := s.DB().QueryRowContext(ctx, `UPDATE outbox_jobs SET available_at=$1 WHERE kind='account_purge' RETURNING id`, stamp(at.Add(time.Hour))).Scan(&id); err != nil {
		t.Fatal(err)
	}
	p := cleanupTestProbe(t, s, u, at, true)
	r := cleanupTestComplete(t, s, p, model.AccountCleanupObservation{SendAction: "blocked", GetChat: "private"}, at)
	if !r.Admitted || r.Generation != 1 {
		t.Fatal(r)
	}
	var got, available string
	var count int
	if err := s.DB().QueryRowContext(ctx, `SELECT id,available_at FROM outbox_jobs WHERE kind='account_purge'`).Scan(&got, &available); err != nil || got != id {
		t.Fatal(got, err)
	}
	when, err := parseStamp(available)
	if err != nil || when.After(at) {
		t.Fatal("fresh final proof remained asleep", when, err)
	}
	if err = s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='account_purge'`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}

func insertOutbox(ctx context.Context, s *Store, kind, payload string, at time.Time) error {
	_, err := hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (bool, error) { return true, insertOutboxTx(ctx, tx, kind, payload, at, at) })
	return err
}

func TestCleanupProtectedFirstOwnerDoesNotStarveOrdinaryScheduling(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	protected := createTestUser(t, s, 497650)
	ordinary := createTestUser(t, s, 497659)
	at := time.Now().UTC().Truncate(time.Microsecond)
	cleanupTestProbe(t, s, protected, at, false)
	cleanupTestProbe(t, s, ordinary, at, false)
	if _, err := s.DB().ExecContext(ctx, `UPDATE account_cleanup_checks SET pending_nonce=NULL,pending_started_at=NULL,pending_final=0,next_probe_at=$1`, at.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfigureAccountCleanupAdmins([]int64{protected.TelegramID}); err != nil {
		t.Fatal(err)
	}
	if err := s.EnqueueAccountCleanupChecks(ctx, at, 1); err != nil {
		t.Fatal(err)
	}
	var owner string
	if err := s.DB().QueryRowContext(ctx, `SELECT payload::jsonb->>'userId' FROM outbox_jobs WHERE kind='account_cleanup_probe'`).Scan(&owner); err != nil || owner != ordinary.ID {
		t.Fatal("configured administrator occupied ordinary scan", owner, err)
	}
}

func TestCleanupAgedWorkerAdmissionRefreshRejectsOldCapturedStart(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497654)
	now := time.Now().UTC().Truncate(time.Microsecond)
	old := cleanupTestProbe(t, s, u, now.Add(-3*time.Minute), false)
	if _, err := s.SetAccountCleanupBot(ctx, old.BotID, now); err != nil {
		t.Fatal(err)
	}
	current, err := s.AccountCleanupProbe(ctx, u.ID, 0, old.Nonce)
	if err != nil || current == nil || current.Nonce != old.Nonce || !current.StartedAt.After(old.StartedAt) {
		t.Fatal("aged backlog did not get bounded API admission", current, err)
	}
	observation := model.AccountCleanupObservation{SendAction: "blocked", GetChat: "private"}
	if r := cleanupTestComplete(t, s, old, observation, current.StartedAt.Add(time.Second)); r.Accepted {
		t.Fatal("old capture completed refreshed admission", r)
	}
	if r := cleanupTestComplete(t, s, *current, observation, current.StartedAt.Add(time.Second)); !r.Accepted || r.Admitted {
		t.Fatal(r)
	}
}
