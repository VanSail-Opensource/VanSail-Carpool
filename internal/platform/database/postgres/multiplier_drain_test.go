package postgres

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestMultiplierRetirementDrainsWritersAndRejectsStaleSnapshots(t *testing.T) {
	ctx, db := multiplierFixture(t)
	stale, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	defer stale.Rollback()
	missing, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		t.Fatal(err)
	}
	defer missing.Rollback()
	var count int
	if err = stale.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err = missing.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	old, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	defer old.Rollback()
	if _, err = old.ExecContext(ctx, `UPDATE xnode_nodes SET name=name WHERE id='multiplier-node'`); err != nil {
		t.Fatal(err)
	}
	if _, err = old.ExecContext(ctx, `UPDATE xnode_report_cursors SET last_sequence=5 WHERE agent_id='multiplier-agent'`); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- Migrate(ctx, db) }()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		err = db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE relation='xnode_nodes'::regclass AND mode='AccessExclusiveLock' AND NOT granted)`).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case err = <-done:
			t.Fatal("migration did not wait for old writer", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
		}
	}
	if err = old.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	// The migration's advisory-lock statement precedes its table-lock wait.
	// READ COMMITTED is essential to observe this writer's final cursor here.
	checkMultiplierValue(t, ctx, db, `SELECT multiplier_fp FROM xnode_node_rates WHERE agent_id='multiplier-agent' AND from_sequence=6`, 1000000)
	checkMultiplierValue(t, ctx, db, `SELECT multiplier_fp FROM xnode_node_rates WHERE agent_id='multiplier-agent' AND from_sequence=5`, 2000000)
	// Mirror the old ingress: node KEY SHARE may follow non-key updates. The
	// mandatory cursor lock must still reject the pre-cutover snapshot.
	var multiplier int64
	err = stale.QueryRowContext(ctx, `SELECT multiplier_fp FROM xnode_nodes WHERE id='multiplier-node' FOR KEY SHARE`).Scan(&multiplier)
	if err == nil {
		err = stale.QueryRowContext(ctx, `SELECT last_sequence FROM xnode_report_cursors WHERE agent_id='multiplier-agent' FOR UPDATE`).Scan(&count)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "40001" {
		t.Fatal("old ingress snapshot crossed the cursor version fence", multiplier, err)
	}
	err = missing.QueryRowContext(ctx, `SELECT multiplier_fp FROM xnode_nodes WHERE id='new-node' FOR KEY SHARE`).Scan(&multiplier)
	if err == nil {
		_, err = missing.ExecContext(ctx, `INSERT INTO xnode_report_cursors(agent_id,node_id,last_sequence,last_digest,updated_at) VALUES('new-agent','new-node',0,'','fixture') ON CONFLICT(agent_id) DO NOTHING`)
	}
	pgErr = nil
	if !errors.As(err, &pgErr) || pgErr.Code != "40001" {
		t.Fatal("old missing-cursor snapshot crossed the insert conflict fence", multiplier, err)
	}
}
