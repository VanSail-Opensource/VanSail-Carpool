package database

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Historical recovery scripts intentionally run against their original columns.
// These columns exist only in this fixture, never in the current runtime schema.
func newHistoricalRewardStore(t *testing.T) *Store {
	t.Helper()
	s := newTestStore(t)
	raw, err := migrations.ReadFile("migrations/048_rollover_calculated_settlement.sql")
	if err != nil {
		t.Fatal(err)
	}
	ddl := strings.SplitN(string(raw), ";", 2)[0]
	ddl = strings.Replace(ddl, "purchase_rollovers_next", "purchase_rollovers", 1)
	for _, q := range []string{
		`ALTER TABLE combos ADD COLUMN rollover_min_remaining_bps INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE purchases ADD COLUMN reward_rollover_min_remaining_bps INTEGER`, ddl,
	} {
		if _, err = s.DB().ExecContext(context.Background(), q); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func seedHistoricalRewardSettlement(t *testing.T, s *Store, id string, now time.Time, processing bool) {
	t.Helper()
	state := "pending"
	if processing {
		state = "processing"
	}
	_, err := s.DB().ExecContext(context.Background(), `INSERT INTO purchase_rollovers(purchase_id,status,traffic_limit_bytes,minimum_remaining_bps,net_paid_txb_minor,created_at,updated_at) SELECT p.id,?,COALESCE(p.entitlement_traffic_limit_bytes,c.traffic_limit_bytes),COALESCE(p.reward_rollover_min_remaining_bps,c.rollover_min_remaining_bps),p.charged_txb_minor,?,? FROM purchases p JOIN combos c ON c.id=p.combo_id WHERE p.id=?`, state, stamp(now), stamp(now), id)
	if err != nil {
		t.Fatal(err)
	}
}
