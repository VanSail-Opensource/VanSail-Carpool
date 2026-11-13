package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

func TestPricingModelActionResumesEveryProductBeyondFormerLimit(t *testing.T) {
	s, now := pricingActionFixture(t, 513)
	seen := map[string]int{}
	optimize := func(ctx context.Context, kind, id string, _ time.Time) error {
		if ctx.Value(pricingModelRevisionKey{}) != "action" {
			t.Fatal("recalibration revision lost across continuation")
		}
		seen[kind+":"+id]++
		return nil
	}
	passes := 0
	for ; passes < 200; passes++ {
		job, err := s.ClaimOutboxJob(t.Context(), now)
		if err != nil || job == nil {
			t.Fatal("remaining targets lost their durable job", job, err)
		}
		before := len(seen)
		err = s.executePricingModelAction(t.Context(), *job, now, optimize)
		if err != nil && !errors.Is(err, outbox.ErrBusy) {
			t.Fatal(err)
		}
		if len(seen)-before > 4 {
			t.Fatal("model action exceeded the bounded work batch")
		}
		if completeErr := s.CompleteOutboxClaim(t.Context(), *job, err, now); completeErr != nil {
			t.Fatal(completeErr)
		}
		if err == nil {
			break
		}
		now = now.Add(2 * time.Second)
	}
	if len(seen) != 513 || passes >= 200 {
		t.Fatal("global model action silently omitted inventory", len(seen), passes)
	}
	for target, count := range seen {
		if count != 1 {
			t.Fatal("continuation restarted an already checkpointed target", target, count)
		}
	}
	var remaining int
	if err := s.db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM outbox_jobs WHERE kind='pricing_model_action'`).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatal("completed action retained work", remaining, err)
	}
}

func TestPricingModelActionRetainsProgressBeforePendingCompute(t *testing.T) {
	s, now := pricingActionFixture(t, 3)
	job, err := s.ClaimOutboxJob(t.Context(), now)
	if err != nil || job == nil {
		t.Fatal(job, err)
	}
	calls := 0
	err = s.executePricingModelAction(t.Context(), *job, now, func(context.Context, string, string, time.Time) error {
		calls++
		if calls == 2 {
			return ErrPricingComputePending
		}
		return nil
	})
	if !errors.Is(err, ErrPricingComputePending) {
		t.Fatal("pending numerical input was marked complete", err)
	}
	wait := &outbox.WaitError{Reason: "awaiting numerical result", Cause: err, After: time.Second}
	if err = s.CompleteOutboxClaim(t.Context(), *job, wait, now); err != nil {
		t.Fatal(err)
	}
	next, err := s.ClaimOutboxJob(t.Context(), now.Add(2*time.Second))
	if err != nil || next == nil || next.Attempts != 1 {
		t.Fatal("dependency wait exhausted delivery budget", next, err)
	}
	var resumed []string
	if err = s.executePricingModelAction(t.Context(), *next, now, func(_ context.Context, _, id string, _ time.Time) error {
		resumed = append(resumed, id)
		return nil
	}); err != nil || len(resumed) != 2 || resumed[0] != "product-0001" || resumed[1] != "product-0002" {
		t.Fatal("pending retry skipped unfinished work or repeated prefix", resumed, err)
	}
}

func TestPricingModelActionCannotCheckpointOrRunFromStaleClaim(t *testing.T) {
	s, now := pricingActionFixture(t, 2)
	job, err := s.ClaimOutboxJob(t.Context(), now)
	if err != nil || job == nil {
		t.Fatal(job, err)
	}
	calls := 0
	optimize := func(context.Context, string, string, time.Time) error {
		calls++
		_, err := s.db.ExecContext(t.Context(), `UPDATE outbox_jobs SET claim_generation=claim_generation+1 WHERE id=$1`, job.ID)
		return err
	}
	if err = s.executePricingModelAction(t.Context(), *job, now, optimize); !errors.Is(err, ErrConflict) {
		t.Fatal("superseded claim advanced cursor", err)
	}
	var payload string
	if err := s.db.QueryRowContext(t.Context(), `SELECT payload FROM outbox_jobs WHERE id=$1`, job.ID).Scan(&payload); err != nil || payload != job.Payload {
		t.Fatal("failed lease fence changed progress", payload, err)
	}
	if err = s.executePricingModelAction(t.Context(), *job, now, optimize); !errors.Is(err, ErrConflict) || calls != 1 {
		t.Fatal("stale retry performed further model work", calls, err)
	}
}

func pricingActionFixture(t *testing.T, count int) (*Store, time.Time) {
	t.Helper()
	s := newTestStore(t)
	now := time.Now().UTC()
	in := PricingIntervention{ID: "action", ScopeType: "global", ScopeID: "all", Command: "RECALIBRATE", ExpiresAt: now.Add(time.Hour)}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(t.Context(), `INSERT INTO pricing_admin_overrides(id,scope_type,scope_id,command,reason,created_by,starts_at,expires_at,expected_version,previous_json,proposed_json,status,created_at)
 VALUES('action','global','all','RECALIBRATE','progress fixture','actor',$1,$2,1,'{}',$3,'ACTIVE',$1)`, pricingStamp(now), pricingStamp(in.ExpiresAt), string(raw)); err != nil {
		t.Fatal(err)
	}
	for index := range count {
		if _, err := s.db.ExecContext(t.Context(), `INSERT INTO pricing_product_settings(product_type,product_id,dynamic_enabled,fixed_price_minor,updated_by,updated_at)
 VALUES('combo',$1,1,100,'actor',$2)`, fmt.Sprintf("product-%04d", index), pricingStamp(now)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.EnqueueOutbox(t.Context(), "pricing_model_action", `{"interventionId":"action"}`, now); err != nil {
		t.Fatal(err)
	}
	return s, now
}
