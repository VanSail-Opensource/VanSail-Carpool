package database

import (
	"context"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func hourlyCreditFixture(t *testing.T) (*Store, model.HourlyCreditRequest) {
	t.Helper()
	s := newTestStore(t)
	u := createTestUser(t, s, 89401)
	ctx := context.Background()
	if err := s.SaveXNodeConfiguration(ctx, xnode.Configuration{ID: "hourly-config", Name: "Hourly fixture", DocumentCipher: "encrypted"}); err != nil {
		t.Fatal(err)
	}
	addHourlyCreditNode(t, s, "hourly-node", "11111111111111111111111111111111")
	var day string
	if err := s.DB().QueryRowContext(ctx, `SELECT (clock_timestamp() AT TIME ZONE 'UTC')::date::text`).Scan(&day); err != nil {
		t.Fatal(err)
	}
	r := model.HourlyCreditRequest{ReservationID: "reservation-one", OwnerID: u.ID, AccountGeneration: u.AccountGeneration, DayUTC: day, NodeID: "hourly-node", AgentID: "11111111111111111111111111111111", BootID: "22222222222222222222222222222222", SessionID: "33333333333333333333333333333333", Kind: "free", ByteCeiling: 1000}
	if _, err := s.EnsureHourlyCreditDay(ctx, r.OwnerID, r.AccountGeneration, day, 1000); err != nil {
		t.Fatal(err)
	}
	return s, r
}

func addHourlyCreditNode(t *testing.T, s *Store, node, agent string) {
	t.Helper()
	ctx := context.Background()
	if err := s.SaveXNodeNode(ctx, xnode.Node{ID: node, Name: "Hourly", ConfigID: "hourly-config", Enabled: true, MultiplierFP: xnode.MultiplierScale, SettingsCipher: "encrypted"}); err != nil {
		t.Fatal(err)
	}
	if err := s.BindXNodeAgent(ctx, node, agent); err != nil {
		t.Fatal(err)
	}
}

func reserveHourlyFixture(t *testing.T, s *Store, r model.HourlyCreditRequest) model.HourlyCreditReservation {
	t.Helper()
	value, err := s.ReserveHourlyCredit(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func readHourlyDayFixture(t *testing.T, s *Store, r model.HourlyCreditRequest) model.HourlyCreditDay {
	t.Helper()
	day, err := s.HourlyCreditDay(context.Background(), r.OwnerID, r.AccountGeneration, r.DayUTC)
	if err != nil {
		t.Fatal(err)
	}
	return day
}

func hourlyProofFixture(t *testing.T, r model.HourlyCreditReservation, up, down, unused, unknown int64, recovery bool) model.HourlyCreditProof {
	t.Helper()
	p := model.HourlyCreditProof{HourlyCreditRequest: r.HourlyCreditRequest, IssueSequence: r.IssueSequence, AdmittedUploadBytes: up, AdmittedDownloadBytes: down, UnusedBytes: unused, UnknownBytes: unknown, Recovery: recovery}
	var err error
	if p.Digest, err = hourlyCreditProofDigest(p); err != nil {
		t.Fatal(err)
	}
	return p
}
