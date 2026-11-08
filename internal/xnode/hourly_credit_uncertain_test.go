package xnode_test

import (
	"context"
	"testing"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestHourlyUnansweredInstallReconcilesSameExpectedBeforeReuse(t *testing.T) {
	f := newHourlyProductionFixture(t, 64<<20)
	socket := f.connect(t, hourlyProductionSession)
	f.demand(t, socket, hourlyProductionSession)
	var install wire.HourlyCreditInstall
	_ = readHourlyProductionCommand(t, socket, wire.HourlyCreditInstallCommand, &install)
	// The agent may have committed this request. Do not answer and require the
	// retry to reconcile the original reservation rather than installing a new ID.
	var seal wire.HourlyCreditSeal
	command := readHourlyProductionCommand(t, socket, wire.HourlyCreditSealCommand, &seal)
	if seal.Expected == nil || *seal.Expected != install {
		t.Fatal("uncertain install did not reconcile its immutable original", seal)
	}
	day, err := f.repo.HourlyCreditDay(context.Background(), f.f.user.ID, f.f.user.AccountGeneration, install.DayUTC)
	if err != nil || day.FreeAvailableBytes() != 0 || day.FreeReservedBytes != int64(install.ByteCeiling) {
		t.Fatal("timeout prematurely returned byte credit", day, err)
	}
	proof := wire.HourlyCreditProof{Install: install, UnusedBytes: install.ByteCeiling}
	proof.Digest = wire.HourlyProofDigest(proof)
	finishHourlyProductionCommand(t, socket, command, wire.HourlyCreditSealCommand, proof)
	var ack wire.HourlyCreditAck
	command = readHourlyProductionCommand(t, socket, wire.HourlyCreditAckCommand, &ack)
	if ack.ReservationID != install.ReservationID || ack.Digest != proof.Digest {
		t.Fatal("uncertain installation settled another identity", ack)
	}
	finishHourlyProductionCommand(t, socket, command, wire.HourlyCreditAckCommand, map[string]bool{"acknowledged": true})
	day, err = f.repo.HourlyCreditDay(context.Background(), f.f.user.ID, f.f.user.AccountGeneration, install.DayUTC)
	if err != nil || day.FreeAvailableBytes() != int64(install.ByteCeiling) || day.FreeAdmittedBytes != 0 || day.FreeReservedBytes != 0 {
		t.Fatal("normal unused proof did not release exactly its own holding", day, err)
	}
}
