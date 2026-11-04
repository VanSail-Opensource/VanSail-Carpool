package database

import (
	"context"
	"testing"
	"time"
)

// Historical grants are seeded as evidence; there is no runtime purchase API.
func seedHistoricalFirewallGrant(t *testing.T, s *Store, user, id string, now time.Time) string {
	t.Helper()
	_, err := s.db.ExecContext(context.Background(), `INSERT INTO xnode_firewall_exemptions(id,user_id,idempotency_key,ip,price_minor,hours,until_at,created_at,issued_version)
 VALUES($1,$2,$1,'8.8.8.8',125,24,$3,$4,(SELECT version FROM xnode_firewall_clock WHERE singleton=1))`, id, user, stamp(now.Add(24*time.Hour)), stamp(now))
	if err != nil {
		t.Fatal(err)
	}
	return id
}
