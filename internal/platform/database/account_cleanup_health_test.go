package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestCleanupBotEpochAndUnavailableFailClosed(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497621)
	at := time.Now().UTC().Truncate(time.Microsecond)
	p := cleanupTestProbe(t, s, u, at, false)
	if _, err := s.SetAccountCleanupBot(ctx, 0, at); err != nil {
		t.Fatal(err)
	}
	if r := cleanupTestComplete(t, s, p, model.AccountCleanupObservation{SendAction: "blocked", GetChat: "private"}, at); r.Accepted || r.Admitted {
		t.Fatal("unverified bot authorized evidence", r)
	}
	h, err := s.SetAccountCleanupBot(ctx, p.BotID+1, at)
	if err != nil || h.Epoch == p.Epoch {
		t.Fatal(h, err)
	}
	if r := cleanupTestComplete(t, s, p, model.AccountCleanupObservation{SendAction: "blocked", GetChat: "private"}, at); r.Accepted {
		t.Fatal("foreign bot evidence accepted", r)
	}
}

func TestCleanupCorrelatedUnknownSurgePausesAdmissionWithoutAccountFailures(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	at := time.Now().UTC().Truncate(time.Microsecond)
	for i := int64(0); i < 5; i++ {
		u := createTestUser(t, s, 497630+i)
		p := cleanupTestProbe(t, s, u, at, false)
		cleanupTestComplete(t, s, p, model.AccountCleanupObservation{SendAction: "unknown", GetChat: "unknown"}, at)
	}
	var paused bool
	var failures int
	if err := s.DB().QueryRowContext(ctx, `SELECT paused_until>$1 FROM account_cleanup_health`, at).Scan(&paused); err != nil || !paused {
		t.Fatal(paused, err)
	}
	if err := s.DB().QueryRowContext(ctx, `SELECT SUM(failure_count) FROM account_cleanup_checks`).Scan(&failures); err != nil || failures != 0 {
		t.Fatal("unknown surge became purge authorization", failures, err)
	}
}

func TestCleanupSameAccountCannotManufactureGlobalSurge(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497640)
	at := time.Now().UTC().Truncate(time.Microsecond)
	for i := 0; i < 6; i++ {
		now := at.Add(time.Duration(i) * time.Second)
		p := cleanupTestProbe(t, s, u, now, false)
		cleanupTestComplete(t, s, p, model.AccountCleanupObservation{SendAction: "unknown", GetChat: "unknown"}, now)
	}
	var checked int
	if err := s.DB().QueryRowContext(ctx, `SELECT checked_count FROM account_cleanup_health`).Scan(&checked); err != nil || checked != 1 {
		t.Fatal(checked, err)
	}
}
