package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestHourlyLockedRetryWaitsForAmbiguousEarlierCommit(t *testing.T) {
	s, request := hourlyCreditFixture(t)
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err = lockHourlyOwnerTx(ctx, tx, request.OwnerID, request.AccountGeneration); err != nil {
		t.Fatal(err)
	}
	if err = lockHourlyCreditNodeTx(ctx, tx, request); err != nil {
		t.Fatal(err)
	}
	if _, err = hourlyCreditDayTx(ctx, tx, request.OwnerID, request.AccountGeneration, request.DayUTC); err != nil {
		t.Fatal(err)
	}
	sequence, err := nextHourlyCreditSequenceTx(ctx, tx, request.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	if err = adjustHourlyCreditDayTx(ctx, tx, request, 0, request.ByteCeiling); err != nil {
		t.Fatal(err)
	}
	if _, err = insertHourlyCreditReservationTx(ctx, tx, request, sequence); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		stored, err := s.ReserveHourlyCredit(ctx, request)
		if err == nil && (stored.IssueSequence != sequence || stored.HourlyCreditRequest != request) {
			err = ErrConflict
		}
		result <- err
	}()
	select {
	case err := <-result:
		t.Fatal("retry bypassed the still-uncertain original owner lock", err)
	case <-time.After(50 * time.Millisecond):
	}
	// The original reply can be lost after this COMMIT. The retry must see its
	// row despite zero available bytes, never declare it definitely unreserved.
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-result:
		if err != nil {
			t.Fatal("ambiguous committed reservation was discarded", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("exact retry did not resume after original COMMIT")
	}
}

func TestHourlyDefiniteInsufficientRetryLeavesResidualAvailable(t *testing.T) {
	s, original := hourlyCreditFixture(t)
	ctx := context.Background()
	addHourlyCreditNode(t, s, "competing-node", "44444444444444444444444444444444")
	competing := original
	competing.ReservationID, competing.NodeID, competing.AgentID, competing.ByteCeiling = "competing", "competing-node", "44444444444444444444444444444444", 500
	r := reserveHourlyFixture(t, s, competing)
	proof := hourlyProofFixture(t, r, 500, 0, 0, 0, false)
	if _, err := s.ApplyHourlyCreditProof(ctx, r.NodeID, r.AgentID, proof); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReserveHourlyCredit(ctx, original); !errors.Is(err, model.ErrHourlyCreditNotReserved) || !errors.Is(err, model.ErrHourlyCreditUnavailable) {
		t.Fatal("locked absent insufficient retry lacked definitive outcome", err)
	}
	if _, err := s.HourlyCreditReservation(ctx, original.ReservationID); !errors.Is(err, model.ErrHourlyCreditNotFound) {
		t.Fatal("definite rejection left a financial mutation", err)
	}
	original.ReservationID, original.ByteCeiling = "residual", 500
	reserveHourlyFixture(t, s, original)
	day := readHourlyDayFixture(t, s, original)
	if day.FreeReservedBytes != 500 || day.FreeAdmittedBytes != 500 || day.FreeAvailableBytes() != 0 {
		t.Fatal("remaining500 bytes could not be used exactly", day)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.ReserveHourlyCredit(cancelled, original); !errors.Is(err, context.Canceled) || errors.Is(err, model.ErrHourlyCreditNotReserved) {
		t.Fatal("context failure authorized dropping an uncertain intent", err)
	}
}
