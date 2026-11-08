package database

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestReorderedHourlyProgressFramesUseOneDeterministicLockOrder(t *testing.T) {
	s, rows := hourlyProgressFixture(t)
	a, b := rows[0], rows[1]
	first := []model.HourlyCreditProgress{{ReservationID: a.ReservationID, IssueSequence: a.IssueSequence, AdmittedUploadBytes: 200}, {ReservationID: b.ReservationID, IssueSequence: b.IssueSequence, AdmittedUploadBytes: 100}}
	second := []model.HourlyCreditProgress{{ReservationID: b.ReservationID, IssueSequence: b.IssueSequence, AdmittedUploadBytes: 150}, {ReservationID: a.ReservationID, IssueSequence: a.IssueSequence, AdmittedUploadBytes: 250}}
	for _, err := range runTogether(func() error {
		_, err := s.RecordHourlyCreditProgressBatch(context.Background(), a.NodeID, a.AgentID, first)
		return err
	}, func() error {
		_, err := s.RecordHourlyCreditProgressBatch(context.Background(), a.NodeID, a.AgentID, second)
		return err
	}) {
		if err != nil {
			t.Fatal(err)
		}
	}
	if day := readHourlyDayFixture(t, s, a.HourlyCreditRequest); day.FreeAdmittedBytes != 400 || day.FreeReservedBytes != 600 {
		t.Fatal("concurrent progress lost or duplicated admitted bytes", day)
	}
}

func TestUnfinalizedHourlyRecoveryPagesDoNotReclaimFinalUnknown(t *testing.T) {
	s, rows := hourlyProgressFixture(t)
	a, b := rows[0], rows[1]
	ctx := context.Background()
	first, err := s.UnfinalizedHourlyCredits(ctx, a.NodeID, a.AgentID, 0, 1)
	if err != nil || len(first) != 1 || first[0].ReservationID != a.ReservationID {
		t.Fatal("first pending page lost original issue order", first, err)
	}
	second, err := s.UnfinalizedHourlyCredits(ctx, a.NodeID, a.AgentID, first[0].IssueSequence, 1)
	if err != nil || len(second) != 1 || second[0].ReservationID != b.ReservationID {
		t.Fatal("pending cursor skipped an obligation", second, err)
	}
	proof := hourlyProofFixture(t, a, 0, 0, 0, a.ByteCeiling, true)
	if _, err = s.ApplyHourlyCreditProof(ctx, a.NodeID, a.AgentID, proof); err != nil {
		t.Fatal(err)
	}
	pending, err := s.UnfinalizedHourlyCredits(ctx, a.NodeID, a.AgentID, 0, 128)
	if err != nil || len(pending) != 1 || pending[0].ReservationID != b.ReservationID {
		t.Fatal("final unknown obligation was proposed for a new absence proof", pending, err)
	}
	if day := readHourlyDayFixture(t, s, a.HourlyCreditRequest); day.FreeReservedBytes != 1000 || day.FreeAvailableBytes() != 0 {
		t.Fatal("pending scan released uncertain credit", day)
	}
	oversized := make([]model.HourlyCreditProgress, maxHourlyCreditProgress+1)
	if _, err = s.RecordHourlyCreditProgressBatch(ctx, a.NodeID, a.AgentID, oversized); !errors.Is(err, ErrConflict) {
		t.Fatal("unbounded progress frame accepted", err)
	}
}
