package xnode

import (
	"context"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

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
