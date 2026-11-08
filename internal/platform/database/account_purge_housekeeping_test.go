package database

import (
	"context"
	"testing"
	"time"
)

func TestHousekeepingCannotDeleteOrdinaryIncompleteAccountOrLiveSession(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	u := createTestUser(t, store, 497850)
	now := time.Now().UTC()
	token := make([]byte, 32)
	if err := store.ReplaceSession(ctx, token, u.ID, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET onboarding_state='intro',created_at=$2 WHERE id=$1`, u.ID, stamp(now.Add(-30*24*time.Hour))); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now); err != nil {
		t.Fatal(err)
	}
	current, err := store.UserBySession(ctx, token, now)
	if err != nil || current.ID != u.ID || current.AccountStatus != "active" {
		t.Fatal("elapsed age or incomplete onboarding removed an ordinary account/session", err)
	}
}
