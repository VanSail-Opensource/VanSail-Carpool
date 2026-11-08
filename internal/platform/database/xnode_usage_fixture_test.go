package database

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

const usageHistoryStream = "11111111111111111111111111111111"

// Native/history tests seed paid scope directly; commerce settlement/debit
// behavior has its independent wallet regressions and is not mocked here.
func usageHistoryFixture(t *testing.T) (*Store, xnode.Identity, xnode.Period, xnode.HourlyPrincipal, time.Time) {
	t.Helper()
	s, identity, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	at := time.Now().UTC().Truncate(time.Microsecond)
	combo := saveTestCombo(t, s, "History", 10, 30)
	if _, err := s.AdjustBalance(ctx, identity.UserID, 100, "history-seed", "fixture", at); err != nil {
		t.Fatal(err)
	}
	purchase := createMemberOperationPurchase(t, s, identity.UserID, combo.ID, "history-purchase", at)
	period := xnode.Period{ID: "history-combo", ProxyUserID: identity.ProxyUserID, PurchaseID: purchase.ID, StartedAt: at.Add(-time.Minute), LimitBytes: 1 << 40, Strategy: "NO_RESET", Status: "open"}
	if err := s.SaveXNodePeriod(ctx, period); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO hourly_enrollments(id,owner_id,account_generation,group_id,activation_key,request_fingerprint,status,created_at,updated_at) VALUES('history-enrollment',$1,0,'history-group','history-key',$2,'preparing',$3,$3)`, identity.UserID, strings.Repeat("a", 64), at); err != nil {
		t.Fatal(err)
	}
	p, err := s.EnsureXNodeHourlyPrincipal(ctx, "history-enrollment", "encrypted")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	receipt, err := payHourlyIntervalTx(ctx, tx, model.HourlyEnrollment{ID: p.EnrollmentID, OwnerID: p.OwnerID, GroupID: p.GroupID}, model.HourlyOffer{HourlyPriceTXBMinor: 1, Revision: 1}, model.HourlyPolicySnapshot{Version: strings.Repeat("a", 64)}, at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE hourly_enrollments SET status='active',activated_at=$1,paid_through=$2 WHERE id=$3`, at, receipt.PaidUntil, p.EnrollmentID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	p, err = s.XNodeHourlyPrincipal(ctx, p.EnrollmentID)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{period.ID, p.PeriodID} {
		if err = s.TrackXNodePeriodNode(ctx, id, "node"); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.RegisterXNodeTrafficStream(ctx, "node", "agent", usageHistoryStream); err != nil {
		t.Fatal(err)
	}
	return s, identity, period, p, at
}

func usageHistoryReport(identity xnode.Identity, period xnode.Period, p xnode.HourlyPrincipal, at time.Time, sequence uint64) wire.TrafficReport {
	return wire.TrafficReport{ID: fmt.Sprintf("agent:%s:%d", usageHistoryStream, sequence), StreamID: usageHistoryStream,
		Samples: []wire.TrafficSample{
			{UserID: int(identity.ProxyUserID), AccountingPeriod: period.ID, From: at, Through: at, UploadBytes: 3, DownloadBytes: 4},
			{UserID: int(p.ProxyUserID), AccountingPeriod: p.PeriodID, From: at, Through: at, UploadBytes: 1},
		}, HourlySamples: []wire.HourlySample{{Owner: wire.HourlyOwner{ID: p.OwnerID, Generation: uint64(p.AccountGeneration)}, EnrollmentID: p.EnrollmentID, UserID: int(p.ProxyUserID), AccountingPeriod: p.PeriodID, DayUTC: at.Format(time.DateOnly), CounterGeneration: 1, From: at, Through: at, UploadBytes: 7, DownloadBytes: 9}}}
}

func commitUsageHistoryFixture(t *testing.T, s *Store, report wire.TrafficReport, batch bool) error {
	t.Helper()
	sequence, err := wire.TrafficSequence("agent", report)
	if err != nil {
		return err
	}
	if !batch {
		return s.RecordXNodeTraffic(context.Background(), "node", "agent", sequence, report.ID, report)
	}
	results, err := s.RecordXNodeTrafficBatch(context.Background(), []xnode.TrafficCommit{{NodeID: "node", AgentID: "agent", Sequence: sequence, Digest: report.ID, Report: report}})
	if err == nil {
		err = results[0]
	}
	return err
}
