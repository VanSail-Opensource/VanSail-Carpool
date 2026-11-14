package database

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestHourlyCreditAccountingSeparatesOpenRecoveryAndHistoricalEvidence(t *testing.T) {
	for _, tt := range []struct {
		name                  string
		reserve, recovery     bool
		historical, finalized bool
		admitted              int64
		pending               bool
		want                  model.HourlyAccountingStatus
	}{
		{name: "empty"},
		{name: "normal open", reserve: true, admitted: 100, pending: true,
			want: model.HourlyAccountingStatus{OpenReservedBytes: "900"}},
		{name: "current recovery unknown", reserve: true, recovery: true, admitted: 100, pending: true,
			want: model.HourlyAccountingStatus{RecoveryUnknownBytes: "900"}},
		{name: "historical recovery unknown", reserve: true, recovery: true, historical: true, admitted: 100, pending: true,
			want: model.HourlyAccountingStatus{HistoricalRecoveryUnknownBytes: "900"}},
		{name: "historical unsealed", reserve: true, historical: true, admitted: 100, pending: true,
			want: model.HourlyAccountingStatus{HistoricalUnsealedReservedBytes: "900", HistoricalUnsealedCount: 1}},
		{name: "historical unsealed fully admitted", reserve: true, historical: true, admitted: 1000, pending: true,
			want: model.HourlyAccountingStatus{HistoricalUnsealedCount: 1}},
		{name: "normal final proof", reserve: true, finalized: true, admitted: 100},
		{name: "recovery fully confirmed", reserve: true, recovery: true, admitted: 1000},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.want = zeroHourlyAccountingBytes(tt.want)
			s, request := hourlyCreditFixture(t)
			ctx := context.Background()
			var before model.HourlyCreditReservation
			if tt.reserve {
				r := reserveHourlyFixture(t, s, request)
				if _, err := s.RecordHourlyCreditProgress(ctx, r.NodeID, r.AgentID,
					model.HourlyCreditProgress{ReservationID: r.ReservationID, IssueSequence: r.IssueSequence, AdmittedDownloadBytes: tt.admitted}); err != nil {
					t.Fatal(err)
				}
				if tt.recovery || tt.finalized {
					proof := hourlyProofFixture(t, r, 0, tt.admitted, r.ByteCeiling-tt.admitted, 0, false)
					if tt.recovery {
						proof = hourlyProofFixture(t, r, 0, 0, 0, r.ByteCeiling, true)
					}
					if _, err := s.ApplyHourlyCreditProof(ctx, r.NodeID, r.AgentID, proof); err != nil {
						t.Fatal(err)
					}
				}
				var err error
				if before, err = s.HourlyCreditReservation(ctx, r.ReservationID); err != nil {
					t.Fatal(err)
				}
			}
			dayBefore := readHourlyDayFixture(t, s, request)
			day := request.DayUTC
			if tt.historical {
				parsed, err := time.Parse(time.DateOnly, day)
				if err != nil {
					t.Fatal(err)
				}
				day = parsed.AddDate(0, 0, 1).Format(time.DateOnly)
			}
			got, pending, err := s.HourlyCreditAccounting(ctx, request.OwnerID, request.AccountGeneration, day)
			if err != nil || got != tt.want || pending != tt.pending {
				t.Fatalf("accounting = %+v, pending=%v, err=%v; want %+v, pending=%v", got, pending, err, tt.want, tt.pending)
			}
			outstanding, err := s.OutstandingHourlyCredits(ctx, request.OwnerID, request.AccountGeneration, "", 1)
			if err != nil || pending != (len(outstanding) != 0) {
				t.Fatal("legacy accountingPending semantics changed", pending, outstanding, err)
			}
			if after := readHourlyDayFixture(t, s, request); after != dayBefore {
				t.Fatal("display read changed accounting day", dayBefore, after)
			}
			if tt.reserve {
				after, err := s.HourlyCreditReservation(ctx, request.ReservationID)
				if err != nil || !reflect.DeepEqual(before, after) {
					t.Fatal("display read changed credit evidence", before, after, err)
				}
			}
			if tt.historical {
				if _, err := s.HourlyCreditDay(ctx, request.OwnerID, request.AccountGeneration, day); !errors.Is(err, ErrNotFound) {
					t.Fatal("display read created the projected day", err)
				}
			}
		})
	}
}

func TestHourlyCreditAccountingRequiresExactOwnerAndGeneration(t *testing.T) {
	s, request := hourlyCreditFixture(t)
	ctx := context.Background()
	reserveHourlyFixture(t, s, request)
	for _, scope := range []struct {
		owner string
		gen   int64
		day   string
		bad   bool
	}{
		{"", 0, request.DayUTC, true},
		{request.OwnerID, -1, request.DayUTC, true},
		{request.OwnerID, 0, "invalid", true},
		{"another-owner", 0, request.DayUTC, false},
		{request.OwnerID, 1, request.DayUTC, false},
	} {
		got, pending, err := s.HourlyCreditAccounting(ctx, scope.owner, scope.gen, scope.day)
		if scope.bad {
			if !errors.Is(err, ErrConflict) {
				t.Fatal("invalid projection scope accepted", scope, err)
			}
		} else if err != nil || pending || got != zeroHourlyAccountingBytes(model.HourlyAccountingStatus{}) {
			t.Fatal("projection crossed owner/generation boundary", scope, got, pending, err)
		}
	}
}

func zeroHourlyAccountingBytes(value model.HourlyAccountingStatus) model.HourlyAccountingStatus {
	for _, field := range []*string{&value.OpenReservedBytes, &value.RecoveryUnknownBytes, &value.HistoricalRecoveryUnknownBytes, &value.HistoricalUnsealedReservedBytes} {
		if *field == "" {
			*field = "0"
		}
	}
	return value
}

func TestHourlyCreditAccountingPreservesAggregateAboveInt64(t *testing.T) {
	s, request := hourlyCreditFixture(t)
	ctx := context.Background()
	owner := createTestUser(t, s, 89409)
	request.OwnerID, request.ReservationID, request.ByteCeiling = owner.ID, "large-free", math.MaxInt64
	if _, err := s.EnsureHourlyCreditDay(ctx, owner.ID, 0, request.DayUTC, math.MaxInt64); err != nil {
		t.Fatal(err)
	}
	// Construct an already-funded test pool; funding authorization is exercised
	// by the commerce suite. Each pool fits int64 while their sum does not.
	if _, err := s.DB().ExecContext(ctx, `UPDATE hourly_credit_days SET extra_funded_bytes=1000 WHERE owner_id=$1 AND account_generation=0 AND day_utc=$2::date`, owner.ID, request.DayUTC); err != nil {
		t.Fatal(err)
	}
	free := reserveHourlyFixture(t, s, request)
	request.ReservationID, request.Kind, request.ByteCeiling = "large-extra", "extra", 1000
	extra := reserveHourlyFixture(t, s, request)
	parsed, err := time.Parse(time.DateOnly, request.DayUTC)
	if err != nil {
		t.Fatal(err)
	}
	nextDay := parsed.AddDate(0, 0, 1).Format(time.DateOnly)
	const total = "9223372036854776807"
	check := func(day string, want model.HourlyAccountingStatus) {
		t.Helper()
		got, pending, err := s.HourlyCreditAccounting(ctx, owner.ID, 0, day)
		if err != nil || !pending || got != zeroHourlyAccountingBytes(want) {
			t.Fatalf("large aggregate = %+v, pending=%v, err=%v; want %+v", got, pending, err, want)
		}
	}
	check(request.DayUTC, model.HourlyAccountingStatus{OpenReservedBytes: total})
	check(nextDay, model.HourlyAccountingStatus{HistoricalUnsealedReservedBytes: total, HistoricalUnsealedCount: 2})
	for _, record := range []model.HourlyCreditReservation{free, extra} {
		proof := hourlyProofFixture(t, record, 0, 0, 0, record.ByteCeiling, true)
		if _, err := s.ApplyHourlyCreditProof(ctx, record.NodeID, record.AgentID, proof); err != nil {
			t.Fatal(err)
		}
	}
	check(request.DayUTC, model.HourlyAccountingStatus{RecoveryUnknownBytes: total})
	check(nextDay, model.HourlyAccountingStatus{HistoricalRecoveryUnknownBytes: total})
}
