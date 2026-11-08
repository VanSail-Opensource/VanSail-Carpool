package database

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func hourlyProgressFixture(t *testing.T) (*Store, []model.HourlyCreditReservation) {
	t.Helper()
	s, first := hourlyCreditFixture(t)
	second := first
	first.ByteCeiling = 600
	second.ReservationID, second.ByteCeiling = "reservation-two", 400
	return s, []model.HourlyCreditReservation{reserveHourlyFixture(t, s, first), reserveHourlyFixture(t, s, second)}
}

func TestHourlyProgressBatchMovesOnlyDirectionalMaxima(t *testing.T) {
	s, rows := hourlyProgressFixture(t)
	ctx := context.Background()
	first, second := rows[0], rows[1]
	batch := []model.HourlyCreditProgress{{ReservationID: first.ReservationID, IssueSequence: first.IssueSequence, AdmittedUploadBytes: 100, AdmittedDownloadBytes: 20}, {ReservationID: second.ReservationID, IssueSequence: second.IssueSequence, AdmittedUploadBytes: 30, AdmittedDownloadBytes: 10}}
	if _, err := s.RecordHourlyCreditProgressBatch(ctx, first.NodeID, first.AgentID, batch); err != nil {
		t.Fatal(err)
	}
	batch[0].AdmittedUploadBytes, batch[0].AdmittedDownloadBytes = 90, 30
	batch[1].AdmittedUploadBytes, batch[1].AdmittedDownloadBytes = 25, 5
	for range 2 {
		if _, err := s.RecordHourlyCreditProgressBatch(ctx, first.NodeID, first.AgentID, batch); err != nil {
			t.Fatal(err)
		}
	}
	day := readHourlyDayFixture(t, s, first.HourlyCreditRequest)
	if day.FreeAdmittedBytes != 170 || day.FreeReservedBytes != 830 || day.FreeAvailableBytes() != 0 {
		t.Fatal("reordered progress minted, duplicated or lost bytes", day)
	}
	wrong := "44444444444444444444444444444444"
	if _, err := s.RecordHourlyCreditProgressBatch(ctx, first.NodeID, wrong, batch); !errors.Is(err, ErrConflict) {
		t.Fatal("foreign authenticated agent altered another obligation", err)
	}
}

func TestInvalidHourlyProgressRollsBackTheWholeFrame(t *testing.T) {
	for _, invalid := range []string{"ceiling", "sequence", "negative", "duplicate"} {
		t.Run(invalid, func(t *testing.T) {
			s, rows := hourlyProgressFixture(t)
			a, b := rows[0], rows[1]
			batch := []model.HourlyCreditProgress{{ReservationID: a.ReservationID, IssueSequence: a.IssueSequence, AdmittedUploadBytes: 10}, {ReservationID: b.ReservationID, IssueSequence: b.IssueSequence, AdmittedUploadBytes: 20}}
			switch invalid {
			case "ceiling":
				batch[1].AdmittedUploadBytes = 401
			case "sequence":
				batch[1].IssueSequence++
			case "negative":
				batch[1].AdmittedDownloadBytes = -1
			case "duplicate":
				batch[1] = batch[0]
			}
			if _, err := s.RecordHourlyCreditProgressBatch(context.Background(), a.NodeID, a.AgentID, batch); !errors.Is(err, ErrConflict) {
				t.Fatal("invalid frame was accepted", invalid, err)
			}
			if day := readHourlyDayFixture(t, s, a.HourlyCreditRequest); day.FreeAdmittedBytes != 0 || day.FreeReservedBytes != 1000 {
				t.Fatal("invalid frame partially committed", day)
			}
		})
	}
}

func TestHourlyBatchStorageFailureDoesNotCommitDailyOrReservationCounters(t *testing.T) {
	s, rows := hourlyProgressFixture(t)
	ctx := context.Background()
	a, b := rows[0], rows[1]
	if _, err := s.DB().ExecContext(ctx, `ALTER TABLE hourly_credit_reservations ADD CONSTRAINT test_hourly_progress_storage CHECK(admitted_upload_bytes<100)`); err != nil {
		t.Fatal(err)
	}
	batch := []model.HourlyCreditProgress{{ReservationID: a.ReservationID, IssueSequence: a.IssueSequence, AdmittedUploadBytes: 1}, {ReservationID: b.ReservationID, IssueSequence: b.IssueSequence, AdmittedUploadBytes: 200}}
	if _, err := s.RecordHourlyCreditProgressBatch(ctx, a.NodeID, a.AgentID, batch); err == nil {
		t.Fatal("failed storage was reported committed")
	}
	if day := readHourlyDayFixture(t, s, a.HourlyCreditRequest); day.FreeAdmittedBytes != 0 || day.FreeReservedBytes != 1000 {
		t.Fatal("failed storage left a daily debit", day)
	}
	if row, err := s.HourlyCreditReservation(ctx, a.ReservationID); err != nil || row.AdmittedUploadBytes != 0 {
		t.Fatal("failed storage advanced another reservation", row, err)
	}
	if _, err := s.DB().ExecContext(ctx, `ALTER TABLE hourly_credit_reservations DROP CONSTRAINT test_hourly_progress_storage`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := s.RecordHourlyCreditProgressBatch(ctx, a.NodeID, a.AgentID, batch); err != nil {
			t.Fatal(err)
		}
	}
	if day := readHourlyDayFixture(t, s, a.HourlyCreditRequest); day.FreeAdmittedBytes != 201 || day.FreeReservedBytes != 799 {
		t.Fatal("storage retry duplicated confirmed admission", day)
	}
}
