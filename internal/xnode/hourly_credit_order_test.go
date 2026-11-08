package xnode_test

import (
	"context"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestHourlyLateLowerIssueSettlesBeforeNewerInstallation(t *testing.T) {
	f := newHourlyProductionFixture(t, 128<<20)
	f.repo.mu.Lock()
	f.repo.beforeReserve = func(ctx context.Context, request model.HourlyCreditRequest) error {
		// Model an older uncertain transaction becoming visible after this
		// peer's initial pending scan, before its newer AgentID issue commits.
		request.ReservationID, request.SessionID, request.ByteCeiling = "late-lower", "44444444444444444444444444444444", 1
		_, err := f.repo.Store.ReserveHourlyCredit(ctx, request)
		return err
	}
	f.repo.mu.Unlock()
	socket := f.connect(t, hourlyProductionSession)
	f.demand(t, socket, hourlyProductionSession)
	var seal wire.HourlyCreditSeal
	command := readHourlyProductionCommand(t, socket, wire.HourlyCreditSealCommand, &seal)
	if seal.Expected == nil || seal.Expected.ReservationID != "late-lower" || seal.Expected.IssueSequence != 1 || seal.Expected.SessionID != "44444444444444444444444444444444" {
		t.Fatal("newer issue hid the original lower obligation", seal)
	}
	proof := wire.HourlyCreditProof{Install: *seal.Expected, UnusedBytes: 1}
	proof.Digest = wire.HourlyProofDigest(proof)
	finishHourlyProductionCommand(t, socket, command, wire.HourlyCreditSealCommand, proof)
	var install wire.HourlyCreditInstall
	command = readHourlyProductionCommand(t, socket, wire.HourlyCreditInstallCommand, &install)
	if install.IssueSequence != 2 || install.SessionID != hourlyProductionSession || install.ReservationID == "late-lower" {
		t.Fatal("new issue retargeted lower scope or broke sequence", install)
	}
	lower, err := f.repo.HourlyCreditReservation(context.Background(), "late-lower")
	if err != nil || lower.FinalProofDigest != proof.Digest {
		t.Fatal("higher issue preceded durable lower settlement", lower, err)
	}
	finishHourlyProductionCommand(t, socket, command, wire.HourlyCreditInstallCommand, map[string]bool{"installed": true})
}
