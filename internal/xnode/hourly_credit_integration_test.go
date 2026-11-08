package xnode_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestHourlyProducerCommitsBeforeDeliveryAndNeverBlocksTrafficACK(t *testing.T) {
	const allowance = 64 << 20
	f := newHourlyProductionFixture(t, allowance)
	socket := f.connect(t, hourlyProductionSession)
	f.demand(t, socket, hourlyProductionSession)
	var install wire.HourlyCreditInstall
	command := readHourlyProductionCommand(t, socket, wire.HourlyCreditInstallCommand, &install)
	ctx := context.Background()
	r, err := f.repo.HourlyCreditReservation(ctx, install.ReservationID)
	if err != nil || r.ByteCeiling != allowance || r.IssueSequence != int64(install.IssueSequence) || r.SessionID != hourlyProductionSession || r.Kind != "free" {
		t.Fatal("install preceded exact durable reservation", r, err)
	}
	day, err := f.repo.HourlyCreditDay(ctx, f.f.user.ID, f.f.user.AccountGeneration, install.DayUTC)
	if err != nil || day.FreeReservedBytes != allowance || day.FreeAvailableBytes() != 0 || day.ExtraFundedBytes != 0 {
		t.Fatal("delivery minted or purchased extra credit", day, err)
	}
	// Leave the install command unanswered while a genuine native journal is
	// committed and acknowledged through the same authenticated TLS socket.
	report := wire.TrafficReport{ID: fixtureAgent + ":1", Traffic: wire.Traffic{}}
	if err = socket.WriteJSON(wire.New("traffic.report", report.ID, 0, report)); err != nil {
		t.Fatal(err)
	}
	if err = socket.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if ack := readFrame(t, socket, "traffic.ack"); ack.ID != report.ID {
		t.Fatal("outstanding install blocked traffic acknowledgment", ack)
	}
	finishHourlyProductionCommand(t, socket, command, wire.HourlyCreditInstallCommand, map[string]bool{"installed": true})
	progress := wire.HourlyCreditProgress{ReservationID: install.ReservationID, IssueSequence: install.IssueSequence, AdmittedUploadBytes: allowance / 4, AdmittedDownloadBytes: allowance * 3 / 4}
	if err = socket.WriteJSON(wire.New("hourly.demand", "", 0, wire.HourlyDemandBatch{SessionID: hourlyProductionSession, Progress: []wire.HourlyCreditProgress{progress}})); err != nil {
		t.Fatal(err)
	}
	var seal wire.HourlyCreditSeal
	sealCommand := readHourlyProductionCommand(t, socket, wire.HourlyCreditSealCommand, &seal)
	if seal.Expected == nil || *seal.Expected != install {
		t.Fatal("fully admitted reservation did not seal its original identity", seal)
	}
	proof := wire.HourlyCreditProof{Install: install, AdmittedUploadBytes: progress.AdmittedUploadBytes, AdmittedDownloadBytes: progress.AdmittedDownloadBytes}
	proof.Digest = wire.HourlyProofDigest(proof)
	finishHourlyProductionCommand(t, socket, sealCommand, wire.HourlyCreditSealCommand, proof)
	var ack wire.HourlyCreditAck
	ackCommand := readHourlyProductionCommand(t, socket, wire.HourlyCreditAckCommand, &ack)
	stored, err := f.repo.HourlyCreditReservation(ctx, install.ReservationID)
	if err != nil || ack.Digest != proof.Digest || stored.FinalProofDigest != proof.Digest || stored.ReservedBytes() != 0 {
		t.Fatal("credit ACK preceded matching durable final proof", stored, ack, err)
	}
	finishHourlyProductionCommand(t, socket, ackCommand, wire.HourlyCreditAckCommand, map[string]bool{"acknowledged": true})
	if err = f.master.CloseHourlyCreditObligations(ctx, f.f.user.ID, -1, ""); err != nil {
		t.Fatal("settled original generation blocked lifecycle", err)
	}
}

func TestHourlyUncertainReserveAndReconnectPreserveOriginalUnknownDebt(t *testing.T) {
	const allowance = 64 << 20
	f := newHourlyProductionFixture(t, allowance)
	f.repo.mu.Lock()
	f.repo.loseReserveReply = true
	f.repo.mu.Unlock()
	first := f.connect(t, hourlyProductionSession)
	f.demand(t, first, hourlyProductionSession)
	var install wire.HourlyCreditInstall
	command := readHourlyProductionCommand(t, first, wire.HourlyCreditInstallCommand, &install)
	finishHourlyProductionCommand(t, first, command, wire.HourlyCreditInstallCommand, map[string]bool{"installed": true})
	var count int
	if err := f.repo.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM hourly_credit_reservations WHERE owner_id=$1`, f.f.user.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("uncertain Reserve minted a second obligation", count, err)
	}
	_ = first.Close()
	waitHourlyProduction(t, func() bool { return !f.master.HourlyCreditAdmissionReady(f.f.node.ID) })
	err := f.master.CloseHourlyCreditObligations(context.Background(), f.f.user.ID, -1, "")
	var policy *xnode.PolicyError
	if !errors.As(err, &policy) || policy.Code != "XNODE_HOURLY_CREDIT_OFFLINE" {
		t.Fatal("disconnect erased or refunded outstanding bytes", err)
	}
	second := f.connect(t, "44444444444444444444444444444444")
	var seal wire.HourlyCreditSeal
	sealCommand := readHourlyProductionCommand(t, second, wire.HourlyCreditSealCommand, &seal)
	if seal.Expected == nil || *seal.Expected != install || seal.Expected.SessionID == "44444444444444444444444444444444" {
		t.Fatal("reconnect retargeted the original financial obligation", seal)
	}
	proof := wire.HourlyCreditProof{Install: install, UnknownBytes: install.ByteCeiling, Recovery: true}
	proof.Digest = wire.HourlyProofDigest(proof)
	finishHourlyProductionCommand(t, second, sealCommand, wire.HourlyCreditSealCommand, proof)
	var ack wire.HourlyCreditAck
	ackCommand := readHourlyProductionCommand(t, second, wire.HourlyCreditAckCommand, &ack)
	finishHourlyProductionCommand(t, second, ackCommand, wire.HourlyCreditAckCommand, map[string]bool{"acknowledged": true})
	err = f.master.CloseHourlyCreditObligations(context.Background(), f.f.user.ID, -1, "")
	if !errors.As(err, &policy) || policy.Code != "XNODE_HOURLY_CREDIT_RECOVERY_UNKNOWN" {
		t.Fatal("recovery unknown was treated as unused", err)
	}
	day, err := f.repo.HourlyCreditDay(context.Background(), f.f.user.ID, f.f.user.AccountGeneration, install.DayUTC)
	if err != nil || day.FreeReservedBytes != allowance || day.FreeAdmittedBytes != 0 || day.FreeAvailableBytes() != 0 {
		t.Fatal("recovery invented usage or returned unknown credit", day, err)
	}
}
