package database

import (
	"context"
	"testing"
	"time"
)

func TestGroupFactBatchRollbackRetainsEveryBufferedFact(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC()
	// Count statements, not rows, to protect the database round-trip reduction.
	for _, query := range []string{
		`CREATE TABLE group_fact_insert_statements(marker boolean NOT NULL)`,
		`CREATE FUNCTION count_group_fact_insert() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN INSERT INTO group_fact_insert_statements VALUES(true); RETURN NULL; END $$`,
		`CREATE TRIGGER count_group_fact_insert AFTER INSERT ON activity_group_message_raw_events FOR EACH STATEMENT EXECUTE FUNCTION count_group_fact_insert()`,
		`CREATE FUNCTION fail_group_fact_insert() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.message_id=513 THEN RAISE EXCEPTION 'injected batch failure'; END IF; RETURN NEW; END $$`,
		`CREATE TRIGGER fail_group_fact_insert BEFORE INSERT ON activity_group_message_raw_events FOR EACH ROW EXECUTE FUNCTION fail_group_fact_insert()`,
	} {
		if _, err := store.DB().ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	for id := int64(1); id <= 1025; id++ {
		if err := store.BufferGroupMessageFact(-100, id, now.Format(time.DateOnly), now); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.FlushGroupMessageFacts(ctx); err == nil {
		t.Fatal("injected second-batch failure was accepted")
	}
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_group_message_raw_events`).Scan(&count); err != nil || count != 0 {
		t.Fatal("failed flush retained a partial batch", count, err)
	}
	if _, err := store.DB().ExecContext(ctx, `DROP TRIGGER fail_group_fact_insert ON activity_group_message_raw_events`); err != nil {
		t.Fatal(err)
	}
	if err := store.FlushGroupMessageFacts(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_group_message_raw_events`).Scan(&count); err != nil || count != 1025 {
		t.Fatal("retry lost buffered facts", count, err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM group_fact_insert_statements`).Scan(&count); err != nil || count != 3 {
		t.Fatal("flush did not use three bounded insert statements", count, err)
	}
	if err := store.FlushGroupMessageFacts(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM group_fact_insert_statements`).Scan(&count); err != nil || count != 3 {
		t.Fatal("completed snapshot was not retired", count, err)
	}
}

func TestGroupFactFlushPreservesConcurrentAdmission(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	store := newTestStore(t)
	now := time.Now().UTC()
	if err := store.BufferGroupMessageFact(-100, 1, now.Format(time.DateOnly), now); err != nil {
		t.Fatal(err)
	}
	blocker, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = blocker.Rollback() }()
	if _, err := blocker.ExecContext(ctx, `LOCK TABLE activity_group_message_raw_events IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- store.FlushGroupMessageFacts(ctx) }()
	waitStoreWriteBlocked(t, ctx, store, "INSERT INTO activity_group_message_raw_events")
	if err := store.BufferGroupMessageFact(-100, 2, now.Format(time.DateOnly), now); err != nil {
		t.Fatal(err)
	}
	if err := blocker.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := store.FlushGroupMessageFacts(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_group_message_raw_events`).Scan(&count); err != nil || count != 2 {
		t.Fatal("concurrent webhook admission was lost", count, err)
	}
}
