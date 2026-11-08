package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func nativePurgeFixture(t *testing.T) (*Store, model.AccountPurge, string, xnode.Identity) {
	t.Helper()
	s, identity, _, _, _ := usageHistoryFixture(t)
	u, err := s.UserByID(context.Background(), identity.UserID)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Truncate(time.Microsecond)
	admitTestAccountCleanup(t, s, u, at)
	target, err := s.AccountPurge(context.Background(), u.ID, 1)
	if err != nil || target == nil {
		t.Fatal("admitted purge missing", target, err)
	}
	var jobID string
	err = s.db.QueryRowContext(context.Background(), `UPDATE outbox_jobs SET status='processing',
 payload=jsonb_set(payload::jsonb,'{retainedTestMarker}','"preserved"'::jsonb)::text
 WHERE kind='account_purge' AND payload::jsonb->>'userId'=$1 AND payload::jsonb->>'generation'='1' RETURNING id`, u.ID).Scan(&jobID)
	if err != nil {
		t.Fatal(err)
	}
	return s, *target, jobID, identity
}
