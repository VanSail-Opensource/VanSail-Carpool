package database

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestHourlyDailyAllowanceIsImmutableAndExtraRequiresFunding(t *testing.T) {
	s, r := hourlyCreditFixture(t)
	ctx := context.Background()
	for _, allowance := range []int64{5000, 100} {
		day, err := s.EnsureHourlyCreditDay(ctx, r.OwnerID, r.AccountGeneration, r.DayUTC, allowance)
		if err != nil || day.FreeAllowanceBytes != 1000 {
			t.Fatal("same-day policy change altered issued allowance", day, err)
		}
	}
	extra := r
	extra.Kind, extra.ByteCeiling = "extra", 500
	if _, err := s.ReserveHourlyCredit(ctx, extra); !errors.Is(err, ErrHourlyCreditUnavailable) {
		t.Fatal("unfunded extra quota issued", err)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE hourly_credit_days SET extra_funded_bytes=500 WHERE owner_id=$1`, r.OwnerID); err != nil {
		t.Fatal(err)
	}
	reserveHourlyFixture(t, s, extra)
	day := readHourlyDayFixture(t, s, r)
	if day.FreeAvailableBytes() != 1000 || day.ExtraReservedBytes != 500 || day.ExtraAvailableBytes() != 0 {
		t.Fatal("free and funded-extra accounting mixed", day)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE hourly_credit_days SET free_allowance_bytes=2000 WHERE owner_id=$1`, r.OwnerID); err == nil {
		t.Fatal("SQL changed the frozen daily allowance")
	}
}

func TestConcurrentNodesCannotReserveTheSameSharedAllowance(t *testing.T) {
	s, first := hourlyCreditFixture(t)
	addHourlyCreditNode(t, s, "hourly-two", "44444444444444444444444444444444")
	second := first
	first.ByteCeiling = 700
	second.ReservationID, second.NodeID, second.AgentID, second.ByteCeiling = "reservation-two", "hourly-two", "44444444444444444444444444444444", 700
	results := runTogether(func() error { _, err := s.ReserveHourlyCredit(context.Background(), first); return err }, func() error { _, err := s.ReserveHourlyCredit(context.Background(), second); return err })
	succeeded := 0
	for _, err := range results {
		if err == nil {
			succeeded++
		} else if !errors.Is(err, ErrHourlyCreditUnavailable) {
			t.Fatal("unexpected reservation conflict", err)
		}
	}
	if day := readHourlyDayFixture(t, s, first); succeeded != 1 || day.FreeReservedBytes != 700 || day.FreeAvailableBytes() != 300 {
		t.Fatal("multi-node reservation minted allowance", succeeded, day)
	}
}

func TestAgentIssueSequenceIsGlobalAcrossOwnersAndReplayIsExact(t *testing.T) {
	s, first := hourlyCreditFixture(t)
	other := createTestUser(t, s, 89402)
	second := first
	second.OwnerID, second.AccountGeneration, second.ReservationID = other.ID, other.AccountGeneration, "reservation-two"
	if _, err := s.EnsureHourlyCreditDay(context.Background(), other.ID, other.AccountGeneration, first.DayUTC, 1000); err != nil {
		t.Fatal(err)
	}
	var values [2]model.HourlyCreditReservation
	results := runTogether(func() error {
		var err error
		values[0], err = s.ReserveHourlyCredit(context.Background(), first)
		return err
	}, func() error {
		var err error
		values[1], err = s.ReserveHourlyCredit(context.Background(), second)
		return err
	})
	for _, err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	if values[0].IssueSequence == values[1].IssueSequence || values[0].IssueSequence+values[1].IssueSequence != 3 {
		t.Fatal("agent-global issue sequence reused", values)
	}
	replay := reserveHourlyFixture(t, s, first)
	if replay.IssueSequence != values[0].IssueSequence || readHourlyDayFixture(t, s, first).FreeReservedBytes != 1000 {
		t.Fatal("reservation replay duplicated capacity", replay)
	}
	changed := first
	changed.SessionID = changed.BootID
	if _, err := s.ReserveHourlyCredit(context.Background(), changed); !errors.Is(err, ErrConflict) {
		t.Fatal("replay replaced the original session", err)
	}
}
