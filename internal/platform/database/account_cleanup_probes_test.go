package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestCleanupGraceSpacingSeparateFinalAndReplay(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497601)
	end := time.Now().UTC().Truncate(time.Microsecond)
	start := end.Add(-7*24*time.Hour - time.Minute)
	if _, err := s.AdjustBalance(ctx, u.ID, 777, "cleanup-grace", "fixture", start); err != nil {
		t.Fatal(err)
	}
	negative := model.AccountCleanupObservation{SendAction: "blocked", GetChat: "private"}
	for _, at := range []time.Time{start, start.Add(time.Hour), start.Add(24 * time.Hour), end.Add(-time.Minute)} {
		p := cleanupTestProbe(t, s, u, at, false)
		result := cleanupTestComplete(t, s, p, negative, at)
		if !result.Accepted || result.Admitted {
			t.Fatal(result)
		}
		if replay := cleanupTestComplete(t, s, p, negative, at); replay.Accepted || replay.Admitted {
			t.Fatal("nonce replay counted", replay)
		}
	}
	got, err := s.UserByID(ctx, u.ID)
	if err != nil || got.AccountStatus != "active" || got.AccountGeneration != 0 {
		t.Fatal("grace mutated lifecycle", got, err)
	}
	if balance, err := s.Balance(ctx, u.ID); err != nil || balance.MinorInt64() != 777 {
		t.Fatal(balance, err)
	}
	var failures int
	var evidence int
	if err = s.DB().QueryRowContext(ctx, `SELECT failure_count FROM account_cleanup_checks WHERE user_id=$1`, u.ID).Scan(&failures); err != nil || failures != 3 {
		t.Fatal(failures, err)
	}
	if err = s.DB().QueryRowContext(ctx, `SELECT count(*) FROM account_cleanup_evidence WHERE user_id=$1`, u.ID).Scan(&evidence); err != nil || evidence != 3 {
		t.Fatal("subday or duplicate observation counted", evidence, err)
	}
	final := cleanupTestProbe(t, s, u, end, true)
	result := cleanupTestComplete(t, s, final, negative, end)
	if !result.Admitted || result.Generation != 1 {
		t.Fatal(result)
	}
	valid, err := s.AccountCleanupAdmissionValid(ctx, u.ID, 1, end)
	if err != nil || !valid {
		t.Fatal(valid, err)
	}
	if valid, err = s.AccountCleanupAdmissionValid(ctx, u.ID, 1, end.Add(6*time.Minute)); err != nil || valid {
		t.Fatal("stale final survived", valid, err)
	}
}

func TestCleanupGenericUnreachableRequiresSameBotPrivateReachability(t *testing.T) {
	for _, prior := range []bool{false, true} {
		t.Run(map[bool]string{false: "never_started", true: "previously_reachable"}[prior], func(t *testing.T) {
			ctx, s := context.Background(), newTestStore(t)
			u := createTestUser(t, s, 497602)
			at := time.Now().UTC().Truncate(time.Microsecond)
			p := cleanupTestProbe(t, s, u, at, false)
			if prior {
				cleanupTestComplete(t, s, p, model.AccountCleanupObservation{SendAction: "reachable", GetChat: "private"}, at)
				p = cleanupTestProbe(t, s, u, at.Add(time.Minute), false)
				at = at.Add(time.Minute)
			}
			cleanupTestComplete(t, s, p, model.AccountCleanupObservation{SendAction: "unreachable", GetChat: "unreachable"}, at)
			var count int
			if err := s.DB().QueryRowContext(ctx, `SELECT failure_count FROM account_cleanup_checks WHERE user_id=$1`, u.ID).Scan(&count); err != nil || count != boolInt(prior) {
				t.Fatal(count, err)
			}
		})
	}
}
