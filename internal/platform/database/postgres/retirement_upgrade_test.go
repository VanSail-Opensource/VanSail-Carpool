package postgres

import (
	"context"
	"database/sql"
	"testing"
)

// Runs only inside the isolated hosted forward-upgrade database at Goose 17.
// Historical financial rows are seeded before retirement rather than recreated
// after the newest schema has already dropped the operational credit tables.
func seedRetirementUpgrade(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	const at = "2026-10-20T00:00:00.000000000Z"
	queries := []string{
		`INSERT INTO users(id,telegram_id,created_at,updated_at) VALUES('retirement-owner',48929,$1,$1)`,
		`INSERT INTO balances(user_id,txb_minor,updated_at) VALUES('retirement-owner',1234,$1)`,
		`INSERT INTO combos(id,name,price_txb_minor,validity_days,traffic_limit_bytes,created_at,updated_at) VALUES('retirement-combo','Historical',100,1,1000,$1,$1)`,
		`INSERT INTO purchases(id,user_id,combo_id,charged_txb_minor,gross_price_txb_minor,valid_from,valid_until,status,created_at,updated_at) SELECT id,'retirement-owner','retirement-combo',100,100,'2026-10-19T00:00:00.000000000Z',$1,'expired',$1,$1 FROM (VALUES('retirement-calculated'),('retirement-settled')) p(id)`,
		`INSERT INTO purchase_rollovers(purchase_id,status,traffic_limit_bytes,minimum_remaining_bps,net_paid_txb_minor,credited_txb_minor,allocated_traffic_bytes,used_traffic_bytes,eligible_unused_bytes,algorithm_version,created_at,updated_at,completed_at) VALUES('retirement-calculated','calculated',1000,0,100,90,1000,100,900,'historical-v1',$1,$1,NULL),('retirement-settled','credited',1000,0,100,25,1000,100,900,'historical-v1',$1,$1,$1)`,
		`INSERT INTO ledger_entries(id,user_id,delta_txb_minor,balance_after,kind,reference_id,note,created_at) VALUES('retirement-ledger','retirement-owner',25,1234,'rollover_credit','retirement-settled','Historical evidence',$1)`,
		`INSERT INTO rollover_member_daily_rollups(local_date,user_id,settlement_count,credited_txb_minor,allocated_traffic_bytes,used_traffic_bytes,updated_at) VALUES('2026-10-20','retirement-owner',1,25,1000,100,$1)`,
		`INSERT INTO outbox_jobs(id,kind,payload,status,available_at,created_at,updated_at) VALUES('retirement-work','rollover_finalize','{"purchaseId":"retirement-calculated"}','processing',$1,$1,$1),('retirement-notice','pricing_renewal_notice','{"purchaseId":"retirement-calculated"}','pending',$1,$1,$1)`,
	}
	for i, q := range queries {
		if _, err := db.ExecContext(ctx, q, at); err != nil {
			t.Fatalf("seed retirement step %d: %v", i, err)
		}
	}
}

func verifyRetirementUpgrade(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	checks := []struct {
		query string
		want  int64
	}{
		{`SELECT txb_minor FROM balances WHERE user_id='retirement-owner'`, 1234},
		{`SELECT COUNT(*) FROM ledger_entries WHERE user_id='retirement-owner'`, 1},
		{`SELECT delta_txb_minor FROM ledger_entries WHERE id='retirement-ledger'`, 25},
		{`SELECT balance_after FROM ledger_entries WHERE id='retirement-ledger'`, 1234},
		{`SELECT SUM(charged_txb_minor) FROM purchases WHERE user_id='retirement-owner'`, 200},
		{`SELECT COUNT(*) FROM purchases WHERE user_id='retirement-owner' AND status='expired'`, 2},
		{`SELECT COUNT(*) FROM outbox_jobs WHERE kind IN ('rollover_finalize','pricing_renewal_notice')`, 0},
		{`SELECT (detail::jsonb->>'walletCreditMinor')::bigint FROM audit_events WHERE id='rollover-retirement'`, 0},
		{`SELECT (detail::jsonb->>'cancelledCalculations')::bigint FROM audit_events WHERE id='rollover-retirement'`, 1},
		{`SELECT (detail::jsonb->>'creditedMinor')::bigint FROM audit_events WHERE id='retired-rollover:retirement-settled'`, 25},
		{`SELECT (detail::jsonb->>'creditedMinor')::bigint FROM audit_events WHERE id='retired-rollover-day:retirement-owner:2026-10-20'`, 25},
	}
	for _, check := range checks {
		var got int64
		if err := db.QueryRowContext(ctx, check.query).Scan(&got); err != nil || got != check.want {
			t.Fatal("retirement changed historical accounting", check.query, got, check.want, err)
		}
	}
}
