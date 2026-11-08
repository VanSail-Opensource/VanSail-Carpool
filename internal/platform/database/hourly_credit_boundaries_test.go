package database

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestHourlyAdmissionRejectsExpiredDayAndStaleLifecycle(t *testing.T) {
	s, request := hourlyCreditFixture(t)
	ctx := context.Background()
	var yesterday string
	if err := s.DB().QueryRowContext(ctx, `SELECT ((clock_timestamp() AT TIME ZONE 'UTC')::date-1)::text`).Scan(&yesterday); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnsureHourlyCreditDay(ctx, request.OwnerID, 0, yesterday, 1000); !errors.Is(err, ErrHourlyCreditExpired) {
		t.Fatal("past allowance was minted", err)
	}
	now := stamp(time.Now().UTC())
	if _, err := s.DB().ExecContext(ctx, `INSERT INTO hourly_credit_days(owner_id,account_generation,day_utc,free_allowance_bytes,created_at,updated_at) VALUES($1,0,$2::date,1000,$3,$3)`, request.OwnerID, yesterday, now); err != nil {
		t.Fatal(err)
	}
	past := request
	past.DayUTC = yesterday
	if _, err := s.ReserveHourlyCredit(ctx, past); !errors.Is(err, ErrHourlyCreditExpired) {
		t.Fatal("expired-day installation admitted", err)
	}
	request.AccountGeneration++
	if _, err := s.ReserveHourlyCredit(ctx, request); !errors.Is(err, ErrAccountUnavailable) {
		t.Fatal("stale/unknown lifecycle admitted", err)
	}
}

func TestHistoricalHourlyProofSettlesAfterAccountClosure(t *testing.T) {
	s, request := hourlyCreditFixture(t)
	ctx := context.Background()
	var yesterday string
	if err := s.DB().QueryRowContext(ctx, `SELECT ((clock_timestamp() AT TIME ZONE 'UTC')::date-1)::text`).Scan(&yesterday); err != nil {
		t.Fatal(err)
	}
	request.DayUTC = yesterday
	now := stamp(time.Now().UTC())
	for _, item := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO hourly_credit_days(owner_id,account_generation,day_utc,free_allowance_bytes,free_reserved_bytes,created_at,updated_at) VALUES($1,0,$2::date,1000,1000,$3,$3)`, []any{request.OwnerID, yesterday, now}},
		{`INSERT INTO hourly_credit_agent_sequences(agent_id,last_issued,updated_at) VALUES($1,1,$2)`, []any{request.AgentID, now}},
		{`INSERT INTO hourly_credit_reservations(reservation_id,owner_id,account_generation,day_utc,node_id,agent_id,boot_id,session_id,kind,byte_ceiling,issue_sequence,created_at) VALUES($1,$2,0,$3::date,$4,$5,$6,$7,'free',1000,1,$8)`, []any{request.ReservationID, request.OwnerID, yesterday, request.NodeID, request.AgentID, request.BootID, request.SessionID, now}},
		{`UPDATE users SET account_status='purged',account_generation=1 WHERE id=$1`, []any{request.OwnerID}},
	} {
		if _, err := s.DB().ExecContext(ctx, item.query, item.args...); err != nil {
			t.Fatal(err)
		}
	}
	r, err := s.HourlyCreditReservation(ctx, request.ReservationID)
	if err != nil {
		t.Fatal(err)
	}
	proof := hourlyProofFixture(t, r, 100, 0, 900, 0, false)
	if _, err = s.ApplyHourlyCreditProof(ctx, r.NodeID, r.AgentID, proof); err != nil {
		t.Fatal("closed account blocked genuine accounting", err)
	}
	if day := readHourlyDayFixture(t, s, request); day.FreeAdmittedBytes != 100 || day.FreeReservedBytes != 0 || day.FreeAvailableBytes() != 900 {
		t.Fatal("historical finalization changed the original day's evidence", day)
	}
}

func TestHourlyCreditSignedCeilingRemainsExact(t *testing.T) {
	s, request := hourlyCreditFixture(t)
	user := createTestUser(t, s, 89403)
	ctx := context.Background()
	request.OwnerID, request.AccountGeneration = user.ID, user.AccountGeneration
	request.ByteCeiling = math.MaxInt64
	if _, err := s.EnsureHourlyCreditDay(ctx, user.ID, user.AccountGeneration, request.DayUTC, math.MaxInt64); err != nil {
		t.Fatal(err)
	}
	r := reserveHourlyFixture(t, s, request)
	p := model.HourlyCreditProgress{ReservationID: r.ReservationID, IssueSequence: r.IssueSequence, AdmittedUploadBytes: math.MaxInt64 - 1, AdmittedDownloadBytes: 1}
	if _, err := s.RecordHourlyCreditProgress(ctx, r.NodeID, r.AgentID, p); err != nil {
		t.Fatal(err)
	}
	proof := hourlyProofFixture(t, r, p.AdmittedUploadBytes, p.AdmittedDownloadBytes, 0, 0, false)
	if _, err := s.ApplyHourlyCreditProof(ctx, r.NodeID, r.AgentID, proof); err != nil {
		t.Fatal(err)
	}
	if day := readHourlyDayFixture(t, s, request); day.FreeAdmittedBytes != math.MaxInt64 || day.FreeReservedBytes != 0 || day.FreeAvailableBytes() != 0 {
		t.Fatal("signed-boundary accounting overflowed", day)
	}
}
