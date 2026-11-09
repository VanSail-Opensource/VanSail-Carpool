package postgres

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestMultiplierRetirementPreservesHistoryAndFencesPolicy(t *testing.T) {
	ctx, db := multiplierFixture(t)
	beforeRates := captureTrafficBoundaryEvidence(t, ctx, db)
	projection := historicalMultiplierProjection(t, ctx, db)
	before, err := inspect(ctx, db, projection)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = owner.ExecContext(ctx, `SELECT pg_advisory_lock(72173680)`); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db); err == nil || !strings.Contains(err.Error(), "Stop the active VanSail runtime") {
		t.Fatal("migration did not refuse active old runtime", err)
	}
	checkMultiplierValue(t, ctx, db, `SELECT multiplier_fp FROM xnode_nodes WHERE id='multiplier-node'`, 1250000)
	if _, err = owner.ExecContext(ctx, `SELECT pg_advisory_unlock(72173680)`); err != nil {
		t.Fatal(err)
	}
	owner.Close()
	if err = Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	after, err := inspect(ctx, db, projection)
	if err != nil || before.Hash != after.Hash {
		t.Fatal("retirement changed authoritative history, financial facts or sequence evidence", err)
	}
	verifyTrafficBoundaryEvidence(t, ctx, db, beforeRates)
	checks := []struct {
		query string
		want  int64
	}{
		{`SELECT count(*) FROM xnode_nodes WHERE multiplier_fp<>1000000`, 0},
		{`SELECT multiplier_fp FROM xnode_node_rates WHERE agent_id='multiplier-agent' AND from_sequence=1`, 1250000},
		{`SELECT multiplier_fp FROM xnode_node_rates WHERE agent_id='multiplier-agent' AND from_sequence=3`, 0},
		{`SELECT multiplier_fp FROM xnode_node_rates WHERE agent_id='multiplier-agent' AND from_sequence=5`, 1000000},
		{`SELECT multiplier_fp FROM xnode_node_rates WHERE agent_id='multiplier-agent' AND from_sequence=9`, 1000000},
		{`SELECT multiplier_fp FROM xnode_node_rates WHERE agent_id='orphan-history' AND from_sequence=1`, 1750000},
		{`SELECT count(*) FROM xnode_node_rates WHERE agent_id='new-agent' AND multiplier_fp<>1000000`, 0},
		{`SELECT count(*) FROM xnode_report_cursors WHERE agent_id NOT IN('multiplier-agent','max-agent')`, 1},
		{`SELECT last_sequence FROM xnode_report_cursors WHERE agent_id='new-agent' AND node_id='new-node' AND last_digest=''`, 0},
		{`SELECT multiplier_fp FROM xnode_stream_rates WHERE stream_id='old-stream' AND from_sequence=1`, 1250000},
		{`SELECT multiplier_fp FROM xnode_stream_rates WHERE stream_id='old-stream' AND from_sequence=4`, 0},
		{`SELECT multiplier_fp FROM xnode_stream_rates WHERE stream_id='old-stream' AND from_sequence=7`, 1000000},
		{`SELECT multiplier_fp FROM xnode_stream_rates WHERE stream_id='old-stream' AND from_sequence=10`, 1000000},
		{`SELECT count(*) FROM xnode_node_rates WHERE agent_id='max-agent'`, 1},
		{`SELECT multiplier_fp FROM xnode_stream_rates WHERE stream_id='max-stream'`, 2500000},
	}
	for _, check := range checks {
		checkMultiplierValue(t, ctx, db, check.query, check.want)
	}
	for _, query := range []string{
		`UPDATE xnode_nodes SET multiplier_fp=0 WHERE id='multiplier-node'`,
		`INSERT INTO xnode_node_rates VALUES('multiplier-agent',12,0)`,
		`UPDATE xnode_node_rates SET multiplier_fp=3000000 WHERE agent_id='multiplier-agent' AND from_sequence=9`,
		`INSERT INTO xnode_stream_rates VALUES('multiplier-agent','old-stream',12,0)`,
		`UPDATE xnode_stream_rates SET multiplier_fp=3000000 WHERE stream_id='old-stream' AND from_sequence=10`,
		`UPDATE xnode_node_rates SET multiplier_fp=multiplier_fp WHERE agent_id='multiplier-agent' AND from_sequence=1`,
		`UPDATE xnode_stream_rates SET multiplier_fp=multiplier_fp WHERE stream_id='old-stream' AND from_sequence=1`,
		`INSERT INTO xnode_node_rates VALUES('multiplier-agent',1,9000000) ON CONFLICT(agent_id,from_sequence) DO UPDATE SET multiplier_fp=excluded.multiplier_fp`,
	} {
		if _, err = db.ExecContext(ctx, query); err != nil {
			t.Fatal("legacy policy compatibility write failed", err)
		}
	}
	checkMultiplierValue(t, ctx, db, `SELECT multiplier_fp FROM xnode_node_rates WHERE agent_id='multiplier-agent' AND from_sequence=1`, 1250000)
	checkMultiplierValue(t, ctx, db, `SELECT count(*) FROM xnode_node_rates WHERE agent_id='multiplier-agent' AND from_sequence>4 AND multiplier_fp<>1000000`, 0)
	checkMultiplierValue(t, ctx, db, `SELECT count(*) FROM xnode_stream_rates WHERE stream_id='old-stream' AND from_sequence>6 AND multiplier_fp<>1000000`, 0)
	for _, query := range []string{
		`UPDATE xnode_node_rates SET multiplier_fp=1000000 WHERE agent_id='multiplier-agent' AND from_sequence=1`,
		`UPDATE xnode_stream_rates SET multiplier_fp=1000000 WHERE stream_id='old-stream' AND from_sequence=1`,
	} {
		if _, err = db.ExecContext(ctx, query); err == nil {
			t.Fatal("committed historical rate accepted mutation")
		}
	}
	if err = Migrate(ctx, db); err != nil {
		t.Fatal("repeated migration changed cutover", err)
	}
	if err = VerifySchema(ctx, db); err != nil {
		t.Fatal(err)
	}
}

func checkMultiplierValue(t *testing.T, ctx context.Context, db *sql.DB, query string, want int64) {
	t.Helper()
	var got int64
	if err := db.QueryRowContext(ctx, query).Scan(&got); err != nil || got != want {
		t.Fatal(query, got, want, err)
	}
}
