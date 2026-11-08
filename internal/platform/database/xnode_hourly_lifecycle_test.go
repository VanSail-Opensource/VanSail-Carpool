package database

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestHourlyIdleFenceCannotRestopNewSameGenerationPreparation(t *testing.T) {
	s, identity, _, p, _ := usageHistoryFixture(t)
	ctx := context.Background()
	if _, err := s.db.ExecContext(ctx, `UPDATE purchases SET status='cancelled' WHERE user_id=$1`, identity.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE hourly_enrollments SET status='disabled' WHERE id=$1`, p.EnrollmentID); err != nil {
		t.Fatal(err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, identity.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO hourly_enrollments(id,owner_id,account_generation,group_id,activation_key,request_fingerprint,status,created_at,updated_at) VALUES('replacement-preparation',$1,0,'history-group','replacement-key',$2,'preparing',clock_timestamp(),clock_timestamp())`, identity.UserID, strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}
	marks := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		values, applied, e := s.WithHourlyOwnerLifecycle(ctx, xnode.HourlyLifecycleIntent{OwnerID: identity.UserID, CurrentGeneration: 0, ThroughGeneration: 0, EnrollmentID: p.EnrollmentID}, func() error { marks <- struct{}{}; return nil })
		if e == nil && (applied || len(values) != 1 || values[0].PeriodID != p.PeriodID) {
			e = xnode.ErrConflict
		}
		done <- e
	}()
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		if err != nil {
			t.Fatal("superseded close lost old scoped cleanup", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("lifecycle guard blocked after owner transaction released")
	}
	select {
	case <-marks:
		t.Fatal("old close re-stopped a new same-generation benefit")
	default:
	}
}

func TestHourlyPurgeCannotBypassReachabilityAdmissionAndDurableCursor(t *testing.T) {
	s, identity, _, p, _ := usageHistoryFixture(t)
	ctx := context.Background()
	intent := xnode.HourlyLifecycleIntent{OwnerID: identity.UserID, CurrentGeneration: 1, ThroughGeneration: 0, Purge: true}
	marks := 0
	mark := func() error { marks++; return nil }
	for _, state := range []struct {
		generation   int
		status, role string
	}{{1, "active", "user"}, {2, "purging", "user"}, {1, "purging", "admin"}} {
		if _, err := s.db.ExecContext(ctx, `UPDATE users SET account_generation=$1,account_status=$2,role=$3 WHERE id=$4`, state.generation, state.status, state.role, identity.UserID); err != nil {
			t.Fatal(err)
		}
		values, applied, err := s.WithHourlyOwnerLifecycle(ctx, intent, mark)
		if !errors.Is(err, xnode.ErrInvalid) || applied || len(values) != 0 || marks != 0 {
			t.Fatal("unauthorized purge fenced native scope", values, applied, marks, err)
		}
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET account_generation=1,account_status='purging',role='user' WHERE id=$1`, identity.UserID); err != nil {
		t.Fatal(err)
	}
	values, applied, err := s.WithHourlyOwnerLifecycle(ctx, intent, mark)
	if !errors.Is(err, xnode.ErrInvalid) || applied || marks != 0 || len(values) != 0 {
		t.Fatal("bare purge bypassed final admission/cursor", values, applied, marks, err)
	}
	current, err := s.XNodeHourlyPrincipal(ctx, p.EnrollmentID)
	if err != nil || current.Status != p.Status {
		t.Fatal("refused bare purge changed the enrollment", current, err)
	}
}
