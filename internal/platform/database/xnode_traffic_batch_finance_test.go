package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestCombinedStorageFailureRollsBackPAYGCharges(t *testing.T) {
	s, parent, combo, a := paygBillingFixture(t)
	ctx := context.Background()
	at := time.Now().UTC()
	item := xnode.TrafficCommit{NodeID: "node", AgentID: "agent", Sequence: 1, Digest: "one", Report: wire.TrafficReport{ID: "agent:1", Samples: []wire.TrafficSample{
		{UserID: int(a.ProxyUserID), AccountingPeriod: a.PeriodID, From: at, Through: at, UploadBytes: uint64(xnode.PAYGStartBytes)},
		{UserID: int(parent.ProxyUserID), AccountingPeriod: combo.ID, From: at, Through: at, UploadBytes: 1},
	}}}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE xnode_daily_usage ADD CONSTRAINT test_finance_rollback CHECK(upload_bytes<1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordXNodeTrafficBatch(ctx, []xnode.TrafficCommit{item}); err == nil {
		t.Fatal("failed combined write was acknowledged")
	}
	account, err := s.XNodePAYGAccount(ctx, a.ProxyUserID)
	if err != nil || account.UsedBytes != 0 || account.PurchasedBytes != 0 || account.BalanceMinor != 1000 {
		t.Fatal("failed commit left a debit or credit", account, err)
	}
	var charges int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM ledger_entries WHERE kind='payg_gb'`).Scan(&charges); err != nil || charges != 0 {
		t.Fatal("failed commit retained a financial fact", charges, err)
	}
	if sequence, err := s.XNodeReportSequence(ctx, "agent"); err != nil || sequence != 0 {
		t.Fatal("failed commit advanced the cursor", sequence, err)
	}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE xnode_daily_usage DROP CONSTRAINT test_finance_rollback`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		outcomes, err := s.RecordXNodeTrafficBatch(ctx, []xnode.TrafficCommit{item})
		if err != nil || len(outcomes) != 1 || outcomes[0] != nil {
			t.Fatal(outcomes, err)
		}
	}
	account, err = s.XNodePAYGAccount(ctx, a.ProxyUserID)
	if err != nil || account.UsedBytes != xnode.PAYGStartBytes || account.PurchasedBytes != xnode.PAYGGB || account.BalanceMinor != 900 {
		t.Fatal("retry lost or repeated the actual charge", account, err)
	}
}

func TestConcurrentFleetBatchesShareWalletLocksAndDeduplication(t *testing.T) {
	s, parent, combo, first := paygBillingFixture(t)
	ctx := context.Background()
	if _, err := s.AdjustBalance(ctx, parent.UserID, -900, "batch-wallet", "seed", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveXNodeNode(ctx, xnode.Node{ID: "node-two", Name: "Second", ConfigID: "config", Enabled: true, MultiplierFP: 10_000_000, SettingsCipher: "encrypted"}); err != nil {
		t.Fatal(err)
	}
	if err := s.BindXNodeAgent(ctx, "node-two", "agent-two"); err != nil {
		t.Fatal(err)
	}
	identity, err := s.SaveXNodeIdentity(ctx, xnode.Identity{ServiceName: "member-payg-batch-second", CredentialsCipher: "encrypted"})
	if err != nil {
		t.Fatal(err)
	}
	second := xnode.PAYGAccount{ProxyUserID: identity.ProxyUserID, UserID: parent.UserID, GroupID: first.GroupID, NodeID: "node-two", PeriodID: "payg-batch-second", UnitPriceMinor: 100}
	if err := s.SaveXNodePeriod(ctx, xnode.Period{ID: second.PeriodID, ProxyUserID: identity.ProxyUserID, StartedAt: time.Now(), Strategy: "PAYG", Status: "open"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveXNodePAYGAccount(ctx, second); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	var items []xnode.TrafficCommit
	for i, account := range []xnode.PAYGAccount{first, second} {
		agent := []string{"agent", "agent-two"}[i]
		id := fmt.Sprintf("%s:1", agent)
		items = append(items, xnode.TrafficCommit{NodeID: account.NodeID, AgentID: agent, Sequence: 1, Digest: id, Report: wire.TrafficReport{ID: id, Samples: []wire.TrafficSample{{UserID: int(account.ProxyUserID), AccountingPeriod: account.PeriodID, From: at, Through: at, UploadBytes: uint64(xnode.PAYGStartBytes)}}}})
	}
	commit := func() error {
		outcomes, err := s.RecordXNodeTrafficBatch(ctx, items)
		if err != nil {
			return err
		}
		for _, outcome := range outcomes {
			if outcome != nil {
				return outcome
			}
		}
		return nil
	}
	for _, err := range runTogether(commit, commit) {
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, account := range []xnode.PAYGAccount{first, second} {
		got, err := s.XNodePAYGAccount(ctx, account.ProxyUserID)
		if err != nil || got.UsedBytes != xnode.PAYGStartBytes || got.PurchasedBytes != xnode.PAYGGB || got.BalanceMinor != -100 {
			t.Fatal("duplicate charge, multiplier or lost delivered usage", got, err)
		}
	}
	var charges int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM ledger_entries WHERE kind='payg_gb'`).Scan(&charges); err != nil || charges != 2 {
		t.Fatal("journal replay repeated financial records", charges, err)
	}
	paid, err := s.XNodePeriod(ctx, combo.ID)
	if err != nil || paid.UsedBytes != 0 {
		t.Fatal("PAYG changed prepaid accounting", paid.UsedBytes, err)
	}
}
