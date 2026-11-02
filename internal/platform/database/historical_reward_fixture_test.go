package database

import (
	"context"

	"testing"
	"time"
)

// Historical recovery scripts intentionally run against their original columns.
// These columns exist only in this fixture, never in the current runtime schema.
func seedHistoricalRewardSettlement(t *testing.T, s *Store, id string, now time.Time, processing bool) {
	t.Helper()
	state := "pending"
	if processing {
		state = "processing"
	}
	_, err := s.DB().ExecContext(context.Background(), `INSERT INTO purchase_rollovers(purchase_id,status,traffic_limit_bytes,minimum_remaining_bps,net_paid_txb_minor,created_at,updated_at) SELECT p.id,$1,COALESCE(p.entitlement_traffic_limit_bytes,c.traffic_limit_bytes),COALESCE(p.reward_rollover_min_remaining_bps,c.rollover_min_remaining_bps),p.charged_txb_minor,$2,$3 FROM purchases p JOIN combos c ON c.id=p.combo_id WHERE p.id=$4`, state, stamp(now), stamp(now), id)
	if err != nil {
		t.Fatal(err)
	}
}
