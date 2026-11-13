package database

import (
	"context"
	"testing"
	"time"
)

func TestAuthTokenRetentionPreservesExactExpiryAndIndependentSessions(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 497724)
	now := time.Date(2026, 11, 12, 14, 0, 0, 0, time.UTC)
	for _, input := range []struct {
		hash             byte
		scope            string
		created, expires time.Time
	}{
		{0, "browser", now.Add(-time.Hour), now},
		{1, "browser", now.Add(-time.Hour), now.Add(900 * time.Nanosecond)},
		{2, "miniapp", now.Add(-time.Hour), now.Add(time.Hour)},
	} {
		_, err := store.DB().ExecContext(ctx, `INSERT INTO sessions(token_hash,user_id,scope,created_at,expires_at,renewed_at)
			VALUES($1,$2,$3,$4,$5,$4)`, []byte{input.hash}, user.ID, input.scope, stamp(input.created), stamp(input.expires))
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, input := range []struct {
		hash    byte
		expires time.Time
	}{{4, now}, {5, now.Add(900 * time.Nanosecond)}} {
		if err := store.CreateBrowserAccessLink(ctx, []byte{input.hash}, user.ID, "/", input.expires); err != nil {
			t.Fatal(err)
		}
	}
	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	counts := make(map[string]int64)
	if err := pruneAuthTokensTx(ctx, tx, now, counts); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if counts["sessions"] != 1 || counts["browser_access_links"] != 1 {
		t.Fatalf("retention counts = %v", counts)
	}
	for _, input := range []struct {
		hash   byte
		exists bool
	}{{0, false}, {1, true}, {2, true}} {
		var exists bool
		if err := store.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sessions WHERE token_hash=$1)`, []byte{input.hash}).Scan(&exists); err != nil || exists != input.exists {
			t.Fatalf("session %d retained = %t, %v; want %t", input.hash, exists, err, input.exists)
		}
	}
	var liveLink bool
	if err := store.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM browser_access_links WHERE token_hash=$1)`, []byte{5}).Scan(&liveLink); err != nil || !liveLink {
		t.Fatalf("live handoff retained = %t, %v", liveLink, err)
	}
}
