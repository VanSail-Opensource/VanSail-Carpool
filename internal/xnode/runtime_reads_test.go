package xnode

import (
	"context"
	"errors"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type paygFundingRepository struct {
	*paygLeaseRepository
	funding []PAYGFunding
	reads   int
	err     error
}

func (r *paygFundingRepository) XNodePAYGFunding(context.Context, string) ([]PAYGFunding, error) {
	r.reads++
	return r.funding, r.err
}

func TestFundingReadsUseFreshProjectionAndPropagateFailure(t *testing.T) {
	_, base, _, _ := compileCacheFixture(t)
	r := &paygFundingRepository{paygLeaseRepository: &paygLeaseRepository{cacheTestRepository: base},
		funding: []PAYGFunding{{ProxyUserID: 8, NodeID: "node", UnitPriceMinor: 100, BalanceMinor: 100}}}
	for _, balance := range []int64{100, -1, 0} {
		r.funding[0].BalanceMinor = balance
		got, err := fundedPAYGAccounts(context.Background(), r, "node")
		if err != nil || len(got) != 1 || got[0].BalanceMinor != balance || r.accountReads != 0 {
			t.Fatal("funding was cached or read historical inventory", got, err, r.accountReads)
		}
	}
	r.err = errors.New("funding read unavailable")
	if _, err := fundedPAYGAccounts(context.Background(), r, "node"); !errors.Is(err, r.err) || r.reads != 4 || r.accountReads != 0 {
		t.Fatal("failed funding read fell back to another snapshot", err, r.reads, r.accountReads)
	}
}

func TestAbsentFundingCannotTurnPAYGIntoOrdinaryAuthorization(t *testing.T) {
	service, base, peer, master := compileCacheFixture(t)
	repo := &paygLeaseRepository{cacheTestRepository: base}
	service.repo = repo
	peer.paygReady = true
	now := time.Now().UTC()
	service.now = func() time.Time { return now }
	master.live[peer.nodeID] = liveSnapshot{receivedAt: now, report: wire.TelemetryReport{Ready: true, Synchronized: true}}
	config := wire.Config{Users: []wire.User{{ID: 7}, {ID: 8, AccountingPeriod: "payg-missing"}}}
	got, err := master.overlayPAYG(context.Background(), peer, config, nil)
	if err != nil || len(got.Users) != 1 || got.Users[0].ID != 7 || len(config.Users) != 2 {
		t.Fatal("missing PAYG account authorized or combo changed", got, err)
	}
}

func TestPAYGPeriodNeverEntersScheduledReset(t *testing.T) {
	period := Period{Strategy: "PAYG", Status: "open", StartedAt: time.Now().Add(-7 * 24 * time.Hour)}
	if cadenceReady(period, time.Now()) {
		t.Fatal("PAYG entered daily fallback reset")
	}
}

func TestIdlePAYGObservationDoesNotReadAccountInventory(t *testing.T) {
	service, base, peer, master := compileCacheFixture(t)
	repo := &paygLeaseRepository{cacheTestRepository: base}
	service.repo = repo
	peer.paygReady = true
	now := time.Now().UTC()
	master.live[peer.nodeID] = liveSnapshot{receivedAt: now, report: wire.TelemetryReport{Ready: true, Synchronized: true}}
	if err := master.observePAYG(context.Background(), []*agentPeer{peer}, 30); err != nil || repo.accountReads != 0 {
		t.Fatal("idle node loaded PAYG inventory", err, repo.accountReads)
	}
}
