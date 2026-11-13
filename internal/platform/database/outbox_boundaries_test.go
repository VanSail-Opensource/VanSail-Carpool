package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestOutboxSchedulingAndRecoveryPreserveExactDeadlines(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	due := at.Add(time.Nanosecond)
	if err := store.EnqueueOutbox(ctx, "precise_schedule", `{"purpose":"deadline"}`, due); err != nil {
		t.Fatal(err)
	}
	if job, err := store.ClaimOutboxJob(ctx, at); err != nil || job != nil {
		t.Fatal("future work started early", job, err)
	}
	job, err := store.ClaimOutboxJob(ctx, due)
	if err != nil || job == nil {
		t.Fatal("due work was not admitted", job, err)
	}
	if err := store.RecoverOutbox(ctx, at, at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if owned, err := store.RenewOutboxClaim(ctx, *job, due); err != nil || !owned {
		t.Fatal("future heartbeat was reclaimed", owned, err)
	}
	if err := store.RecoverOutbox(ctx, due, at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if owned, err := store.RenewOutboxClaim(ctx, *job, due); err != nil || !owned {
		t.Fatal("equal heartbeat was reclaimed", owned, err)
	}
	if err := store.RecoverOutbox(ctx, due.Add(time.Nanosecond), due); err != nil {
		t.Fatal(err)
	}
	if owned, err := store.RenewOutboxClaim(ctx, *job, due); err != nil || owned {
		t.Fatal("expired claim retained authority", owned, err)
	}
	current, err := store.ClaimOutboxJob(ctx, due)
	if err != nil || current == nil || current.ID != job.ID || current.ClaimGeneration <= job.ClaimGeneration {
		t.Fatal("recovery did not fence its previous claim", current, err)
	}
}

func TestOutboxFailureRetainsValidBoundedUnicodeEvidence(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	at := time.Now().UTC()
	for i, text := range []string{strings.Repeat("x", 499) + "故障", "invalid bytes: " + string([]byte{0xff, 0})} {
		if err := store.EnqueueOutbox(ctx, "unicode_failure", fmt.Sprintf(`{"sequence":%d}`, i), at); err != nil {
			t.Fatal(err)
		}
		job, err := store.ClaimOutboxJob(ctx, at)
		if err != nil || job == nil {
			t.Fatal("claim", job, err)
		}
		if err := store.CompleteOutboxClaim(ctx, *job, errors.New(text), at); err != nil {
			t.Fatal("error evidence prevented durable completion", err)
		}
		var status, message string
		if err := store.DB().QueryRowContext(ctx, `SELECT status,last_error FROM outbox_jobs WHERE id=$1`, job.ID).Scan(&status, &message); err != nil || status != "pending" || message == "" || len(message) > 500 || !utf8.ValidString(message) {
			t.Fatal("invalid retained failure", status, message, err)
		}
	}
}

func TestOutboxDeletionRechecksCurrentAdministrator(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	actor := createTestAdmin(t, store, 497715)
	if err := store.EnqueueOutbox(ctx, "optional_artifact", `{"purpose":"authority"}`, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := store.DB().QueryRowContext(ctx, `SELECT id FROM outbox_jobs WHERE kind='optional_artifact'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='user' WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteOutboxJob(ctx, id, actor.ID); !errors.Is(err, ErrConflict) {
		t.Fatal("demoted actor deleted queued work", err)
	}
	var retained, audited int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE id=$1`, id).Scan(&retained); err != nil || retained != 1 {
		t.Fatal("rejected deletion changed the queue", retained, err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE action='outbox.delete' AND target_id=$1`, id).Scan(&audited); err != nil || audited != 0 {
		t.Fatal("rejected deletion created success evidence", audited, err)
	}
}
