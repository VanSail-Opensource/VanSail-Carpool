package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestPAYGRetentionCommitsBoundedBatchesWithoutAccountingChanges(t *testing.T) {
	s, parent, _, account := paygBillingFixture(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Minute)
	before, err := s.XNodePAYGAccount(ctx, account.ProxyUserID)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for i := 0; i < paygRetentionBatch+1; i++ {
		if _, err := tx.ExecContext(ctx, `INSERT INTO xnode_payg_minutes(user_id,group_id,minute,traffic_bytes) VALUES(?,?,?,1)`, parent.UserID, fmt.Sprintf("expired-%d", i), stamp(now.Add(-73*time.Hour))); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO xnode_payg_windows(proxy_user_id,through,traffic_bytes,unbilled_bytes) VALUES(?,?,1,1)`, account.ProxyUserID, paygWindowStamp(now.Add(-73*time.Hour).Add(time.Duration(i)))); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	batch, err := s.PrunePAYGTransientBatch(ctx, now)
	if err != nil || batch["xnode_payg_minutes"] != paygRetentionBatch || batch["xnode_payg_windows"] != paygRetentionBatch {
		t.Fatal(batch, err)
	}
	counts, err := s.PrunePAYGTransientRecords(ctx, now)
	if err != nil || counts["xnode_payg_minutes"] != 1 || counts["xnode_payg_windows"] != 1 {
		t.Fatal(counts, err)
	}
	again, err := s.PrunePAYGTransientRecords(ctx, now)
	if err != nil || again["xnode_payg_minutes"] != 0 || again["xnode_payg_windows"] != 0 {
		t.Fatal(again, err)
	}
	after, err := s.XNodePAYGAccount(ctx, account.ProxyUserID)
	if err != nil || before != after {
		t.Fatal("accounting changed", before, after, err)
	}
}

func TestPAYGRetentionKeepsExactWindowBoundaries(t *testing.T) {
	s, parent, _, account := paygBillingFixture(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Minute)
	cutoff := now.Add(-xnode.PAYGWindow)
	for i, at := range []time.Time{cutoff.Add(-time.Minute), cutoff, now} {
		if _, err := s.DB().ExecContext(ctx, `INSERT INTO xnode_payg_minutes(user_id,group_id,minute) VALUES(?,?,?)`, parent.UserID, fmt.Sprint(i), stamp(at)); err != nil {
			t.Fatal(err)
		}
	}
	for _, at := range []time.Time{cutoff.Add(-time.Nanosecond), cutoff, cutoff.Add(time.Nanosecond)} {
		if _, err := s.DB().ExecContext(ctx, `INSERT INTO xnode_payg_windows(proxy_user_id,through) VALUES(?,?)`, account.ProxyUserID, paygWindowStamp(at)); err != nil {
			t.Fatal(err)
		}
	}
	repairCtx, cancel := context.WithCancel(ctx)
	cancel()
	if err := s.EnqueueXNodePAYGRepairs(repairCtx, now); err == nil {
		t.Fatal("cancelled repair unexpectedly succeeded")
	}
	counts, err := s.PrunePAYGTransientRecords(ctx, now)
	if err != nil || counts["xnode_payg_minutes"] != 1 || counts["xnode_payg_windows"] != 2 {
		t.Fatal(counts, err)
	}
	if err := s.EnqueueXNodePAYGRepairs(ctx, now); err != nil {
		t.Fatal(err)
	}
}
