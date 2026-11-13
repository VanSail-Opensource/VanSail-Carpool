package database

import (
	"context"
	"testing"
	"time"
)

func TestSessionRetentionUsesBoundedDeletesAndPreservesLiveBearer(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	owner := createTestUser(t, store, 497727)
	now := time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)
	if _, err := store.DB().ExecContext(ctx, `INSERT INTO sessions(token_hash,user_id,scope,expires_at,created_at,renewed_at)
		SELECT decode(lpad(to_hex(i),8,'0'),'hex'),$1,'browser',$2,$3,$3 FROM generate_series(1,1025) AS ids(i)`,
		owner.ID, stamp(now), stamp(now.Add(-time.Hour))); err != nil {
		t.Fatal(err)
	}
	live := []byte("live-independent-device")
	if err := store.CreateBrowserSession(ctx, live, owner.ID, now.Add(-time.Hour), now.Add(time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`CREATE TABLE session_delete_statements(marker boolean NOT NULL)`,
		`CREATE FUNCTION count_session_delete() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN INSERT INTO session_delete_statements VALUES(true); RETURN NULL; END $$`,
		`CREATE TRIGGER count_session_delete AFTER DELETE ON sessions FOR EACH STATEMENT EXECUTE FUNCTION count_session_delete()`,
	} {
		if _, err := store.DB().ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	counts := map[string]int64{}
	if err := pruneAuthTokensTx(ctx, tx, now, counts); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if counts["sessions"] != 1025 {
		t.Fatal("expired bearer count changed", counts)
	}
	var statements, retained int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM session_delete_statements`).Scan(&statements); err != nil || statements != 3 {
		t.Fatal("retention did not batch expired bearers", statements, err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE token_hash=$1`, live).Scan(&retained); err != nil || retained != 1 {
		t.Fatal("live final nanosecond was removed", retained, err)
	}
}
