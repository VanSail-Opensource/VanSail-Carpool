package xnode_test

import (
	"context"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestHourlyDefinitivelyUnreservedIntentCanUseSmallerResidual(t *testing.T) {
	f := newHourlyProductionFixture(t, 1000)
	original := make(chan model.HourlyCreditRequest, 1)
	f.repo.mu.Lock()
	f.repo.beforeReserve = func(ctx context.Context, request model.HourlyCreditRequest) error {
		original <- request
		competing := request
		competing.ReservationID, competing.ByteCeiling = "already-consumed", 500
		r, err := f.repo.Store.ReserveHourlyCredit(ctx, competing)
		if err != nil {
			return err
		}
		install := wire.HourlyCreditInstall{ReservationID: r.ReservationID, IssueSequence: uint64(r.IssueSequence), Owner: wire.HourlyOwner{ID: r.OwnerID, Generation: uint64(r.AccountGeneration)}, DayUTC: r.DayUTC, AgentID: r.AgentID, BootID: r.BootID, SessionID: r.SessionID, ByteCeiling: uint64(r.ByteCeiling), Kind: r.Kind}
		proof := wire.HourlyCreditProof{Install: install, AdmittedUploadBytes: 500}
		_, err = f.repo.Store.ApplyHourlyCreditProof(ctx, r.NodeID, r.AgentID, model.HourlyCreditProof{HourlyCreditRequest: r.HourlyCreditRequest, IssueSequence: r.IssueSequence, AdmittedUploadBytes: 500, Digest: wire.HourlyProofDigest(proof)})
		return err
	}
	f.repo.mu.Unlock()
	socket := f.connect(t, hourlyProductionSession)
	f.demand(t, socket, hourlyProductionSession)
	var install wire.HourlyCreditInstall
	command := readHourlyProductionCommand(t, socket, wire.HourlyCreditInstallCommand, &install)
	request := <-original
	if request.ByteCeiling != 1000 || install.ByteCeiling != 500 || install.ReservationID == request.ReservationID {
		t.Fatal("definitively absent original blocked or mutated residual delivery", request, install)
	}
	finishHourlyProductionCommand(t, socket, command, wire.HourlyCreditInstallCommand, map[string]bool{"installed": true})
	day, err := f.repo.HourlyCreditDay(context.Background(), f.f.user.ID, f.f.user.AccountGeneration, install.DayUTC)
	if err != nil || day.FreeAdmittedBytes != 500 || day.FreeReservedBytes != 500 || day.FreeAvailableBytes() != 0 {
		t.Fatal("residual retry changed exact day conservation", day, err)
	}
}
