package xnode_test

import (
	"testing"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestHourlyConfirmedLowWaterProgressPrefetchesBeforePayloadBlocks(t *testing.T) {
	f := newHourlyProductionFixture(t, 1<<30)
	socket := f.connect(t, hourlyProductionSession)
	f.demand(t, socket, hourlyProductionSession)
	var first wire.HourlyCreditInstall
	command := readHourlyProductionCommand(t, socket, wire.HourlyCreditInstallCommand, &first)
	finishHourlyProductionCommand(t, socket, command, wire.HourlyCreditInstallCommand, map[string]bool{"installed": true})
	progress := wire.HourlyCreditProgress{ReservationID: first.ReservationID, IssueSequence: first.IssueSequence, AdmittedUploadBytes: first.ByteCeiling * 3 / 4}
	idle := wire.HourlyDemand{Owner: first.Owner, Nonce: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", LifetimeMillis: 10000}
	// The client still has usable full-speed credit. PendingBytes remains zero;
	// durably confirmed consumption, rather than a blocked payload, wakes refill.
	if err := socket.WriteJSON(wire.New("hourly.demand", "", 0, wire.HourlyDemandBatch{SessionID: hourlyProductionSession, Demands: []wire.HourlyDemand{idle}, Progress: []wire.HourlyCreditProgress{progress}})); err != nil {
		t.Fatal(err)
	}
	var next wire.HourlyCreditInstall
	command = readHourlyProductionCommand(t, socket, wire.HourlyCreditInstallCommand, &next)
	if next.Owner != first.Owner || next.ReservationID == first.ReservationID || next.IssueSequence <= first.IssueSequence || next.ByteCeiling == 0 {
		t.Fatal("progress-only low-water prefetch lost the original owner or issue order", next)
	}
	finishHourlyProductionCommand(t, socket, command, wire.HourlyCreditInstallCommand, map[string]bool{"installed": true})
}
