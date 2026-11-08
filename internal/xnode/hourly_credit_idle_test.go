package xnode_test

import (
	"context"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestHourlyIdleZeroOrUnknownDemandDoesNotPollCreditLedger(t *testing.T) {
	f := newHourlyProductionFixture(t, 1000)
	socket := f.connect(t, hourlyProductionSession)
	waitHourlyProduction(t, func() bool { return f.repo.scans.Load() > 0 })
	baseline := f.repo.scans.Load()
	owner := wire.HourlyOwner{ID: f.f.user.ID, Generation: uint64(f.f.user.AccountGeneration)}
	for n := 0; n < 8; n++ {
		demands := []wire.HourlyDemand{
			{Owner: owner, Nonce: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", LifetimeMillis: 10000},
			{Owner: wire.HourlyOwner{ID: "unknown-owner", Generation: 0}, Nonce: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", LifetimeMillis: 10000, PendingBytes: 1000, MinAtomicBytes: 1},
		}
		if err := socket.WriteJSON(wire.New("hourly.demand", "", 0, wire.HourlyDemandBatch{SessionID: hourlyProductionSession, Demands: demands})); err != nil {
			t.Fatal(err)
		}
	}
	// A quiet observation window spans several credit scheduler ticks. No idle
	// SQL poll is necessary, regardless of repeated unauthoritative keepalives.
	timer := time.NewTimer(400 * time.Millisecond)
	defer timer.Stop()
	<-timer.C
	if after := f.repo.scans.Load(); after != baseline {
		t.Fatal("zero/unknown demand restarted idle SQL polling", baseline, after)
	}
	var count int
	if err := f.repo.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM hourly_credit_reservations`).Scan(&count); err != nil || count != 0 {
		t.Fatal("idle or unknown owner minted credit", count, err)
	}
	// Legitimate positive demand must still restart allocation on this exact
	// connection after the idle fast path, without waiting for a reconnect.
	f.demand(t, socket, hourlyProductionSession)
	var install wire.HourlyCreditInstall
	command := readHourlyProductionCommand(t, socket, wire.HourlyCreditInstallCommand, &install)
	if install.Owner != owner || install.ByteCeiling != 1000 {
		t.Fatal("idle suppression blocked legitimate funded prefetch", install)
	}
	finishHourlyProductionCommand(t, socket, command, wire.HourlyCreditInstallCommand, map[string]bool{"installed": true})
}
