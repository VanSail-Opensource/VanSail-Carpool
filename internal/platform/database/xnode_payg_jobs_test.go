package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestPAYGRepairPageSkipsOwnersThatCannotBeProvisioned(t *testing.T) {
	s, input := paygPreparationFixture(t)
	ctx, now := context.Background(), time.Now().UTC()
	if err := s.SaveXNodeGroup(ctx, xnode.Group{ID: input.GroupID, Name: "Repair group", Enabled: true,
		Bindings: []xnode.GroupBinding{{NodeID: input.NodeID, InboundTag: "paid"}}}); err != nil {
		t.Fatal(err)
	}
	// These 64 lexical predecessors would occupy every slot forever while the
	// worker rejects their lifecycle or bot-blocked state before provisioning.
	if _, err := s.db.ExecContext(ctx, `INSERT INTO users(id,telegram_id,onboarding_state,account_status,bot_blocked,created_at,updated_at)
 SELECT '000-ineligible-'||lpad(i::text,3,'0'),980000+i,'complete',
 CASE i%3 WHEN 0 THEN 'purging' WHEN 1 THEN 'purged' ELSE 'active' END,
 CASE WHEN i%3=2 THEN 1 ELSE 0 END,$1,$1 FROM generate_series(1,64) i`, stamp(now)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO users(id,telegram_id,onboarding_state,created_at,updated_at)
 VALUES('zz-eligible-repair-owner',981000,'complete',$1,$1)`, stamp(now)); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.EnqueueXNodePAYGRepairs(ctx, now); err != nil {
			t.Fatal(err)
		}
	}
	var eligible, unavailable int
	if err := s.db.QueryRowContext(ctx, `SELECT
 COUNT(*) FILTER (WHERE payload::jsonb->>'userId'='zz-eligible-repair-owner'),
 COUNT(*) FILTER (WHERE payload::jsonb->>'userId' LIKE '000-ineligible-%')
 FROM outbox_jobs WHERE kind=$1 AND status IN ('pending','processing')`, xnode.PAYGJob).Scan(&eligible, &unavailable); err != nil {
		t.Fatal(err)
	}
	if eligible != 1 || unavailable != 0 {
		t.Fatal("unavailable owners starved or duplicated eligible repair", eligible, unavailable)
	}
}
