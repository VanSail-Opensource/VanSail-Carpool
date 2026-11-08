package database

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestNormalHourlyProofReturnsOnlyProvenUnusedAndIsImmutable(t *testing.T) {
	s, request := hourlyCreditFixture(t)
	r := reserveHourlyFixture(t, s, request)
	ctx := context.Background()
	if _, err := s.RecordHourlyCreditProgress(ctx, r.NodeID, r.AgentID, model.HourlyCreditProgress{ReservationID: r.ReservationID, IssueSequence: r.IssueSequence, AdmittedUploadBytes: 200, AdmittedDownloadBytes: 100}); err != nil {
		t.Fatal(err)
	}
	proof := hourlyProofFixture(t, r, 250, 150, 600, 0, false)
	for range 2 {
		result, err := s.ApplyHourlyCreditProof(ctx, r.NodeID, r.AgentID, proof)
		if err != nil || result.FinalProofDigest != proof.Digest || result.UnusedBytes != 600 || result.ReservedBytes() != 0 {
			t.Fatal("final unused proof failed", result, err)
		}
	}
	day := readHourlyDayFixture(t, s, request)
	if day.FreeAdmittedBytes != 400 || day.FreeReservedBytes != 0 || day.FreeAvailableBytes() != 600 {
		t.Fatal("normal proof did not conserve shared credit", day)
	}
	changed := hourlyProofFixture(t, r, 200, 150, 650, 0, false)
	if _, err := s.ApplyHourlyCreditProof(ctx, r.NodeID, r.AgentID, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("disagreeing final proof replaced a receipt", err)
	}
	if _, err := s.RecordHourlyCreditProgress(ctx, r.NodeID, r.AgentID, model.HourlyCreditProgress{ReservationID: r.ReservationID, IssueSequence: r.IssueSequence, AdmittedUploadBytes: 251}); !errors.Is(err, ErrConflict) {
		t.Fatal("post-final progress exceeded a normal final partition", err)
	}
	if _, err := s.DB().ExecContext(ctx, `DELETE FROM hourly_credit_reservations WHERE reservation_id=$1`, r.ReservationID); err == nil {
		t.Fatal("immutable reservation evidence was deleted")
	}
}

func TestRecoveryCombinesSavedAndLateProgressWithoutReleasingUnknown(t *testing.T) {
	s, request := hourlyCreditFixture(t)
	r := reserveHourlyFixture(t, s, request)
	ctx := context.Background()
	p := model.HourlyCreditProgress{ReservationID: r.ReservationID, IssueSequence: r.IssueSequence, AdmittedUploadBytes: 100, AdmittedDownloadBytes: 50}
	if _, err := s.RecordHourlyCreditProgress(ctx, r.NodeID, r.AgentID, p); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE users SET account_status='purged',account_generation=account_generation+1 WHERE id=$1`, r.OwnerID); err != nil {
		t.Fatal(err)
	}
	proof := hourlyProofFixture(t, r, 0, 0, 0, 1000, true)
	result, err := s.ApplyHourlyCreditProof(ctx, r.NodeID, r.AgentID, proof)
	if err != nil || result.UnknownBytes != 850 || result.UnusedBytes != 0 || result.AdmittedUploadBytes != 100 || result.AdmittedDownloadBytes != 50 {
		t.Fatal("restart lost saved progress or refunded uncertainty", result, err)
	}
	p.AdmittedUploadBytes, p.AdmittedDownloadBytes = 150, 100
	for range 2 {
		result, err = s.RecordHourlyCreditProgress(ctx, r.NodeID, r.AgentID, p)
		if err != nil || result.UnknownBytes != 750 || result.FinalProofDigest != proof.Digest {
			t.Fatal("late progress rewrote the proof or lost unknown obligations", result, err)
		}
	}
	if _, err = s.ApplyHourlyCreditProof(ctx, r.NodeID, r.AgentID, proof); err != nil {
		t.Fatal("same raw recovery proof could not replay after projection advanced", err)
	}
	if day := readHourlyDayFixture(t, s, request); day.FreeAdmittedBytes != 250 || day.FreeReservedBytes != 750 || day.FreeAvailableBytes() != 0 {
		t.Fatal("recovery increased available allowance", day)
	}
	if _, err = s.ReserveHourlyCredit(ctx, request); !errors.Is(err, ErrAccountUnavailable) {
		t.Fatal("closed account admitted another installation", err)
	}
}

func TestHourlyProofRequiresExactOriginalScopeAndAuthenticatedNode(t *testing.T) {
	s, request := hourlyCreditFixture(t)
	r := reserveHourlyFixture(t, s, request)
	ctx := context.Background()
	proof := hourlyProofFixture(t, r, 0, 0, 1000, 0, false)
	if _, err := s.ApplyHourlyCreditProof(ctx, "foreign-node", r.AgentID, proof); !errors.Is(err, ErrConflict) {
		t.Fatal("foreign authenticated node settled an obligation", err)
	}
	changed := proof
	changed.SessionID = changed.BootID
	var err error
	changed.Digest, err = hourlyCreditProofDigest(changed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyHourlyCreditProof(ctx, r.NodeID, r.AgentID, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("new session relabeled the original obligation", err)
	}
	if day := readHourlyDayFixture(t, s, request); day.FreeReservedBytes != 1000 || day.FreeAvailableBytes() != 0 {
		t.Fatal("rejected proof released reserved bytes", day)
	}
}
