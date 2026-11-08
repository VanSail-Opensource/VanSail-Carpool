package database

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestHourlyUnusedBytesReachOtherNodeOnlyAfterAuthenticatedProof(t *testing.T) {
	s, request := hourlyCreditFixture(t)
	ctx := context.Background()
	request.ByteCeiling = 700
	original := reserveHourlyFixture(t, s, request)
	addHourlyCreditNode(t, s, "other-node", "44444444444444444444444444444444")
	other := request
	other.ReservationID, other.NodeID, other.AgentID, other.ByteCeiling = "other-install", "other-node", "44444444444444444444444444444444", 900
	if _, err := s.ReserveHourlyCredit(ctx, other); !errors.Is(err, model.ErrHourlyCreditUnavailable) {
		t.Fatal("another node reused still-held bytes", err)
	}
	proof := hourlyProofFixture(t, original, 100, 0, 600, 0, false)
	if _, err := s.ApplyHourlyCreditProof(ctx, other.NodeID, other.AgentID, proof); !errors.Is(err, ErrConflict) {
		t.Fatal("foreign authenticated peer returned original node credit", err)
	}
	if day := readHourlyDayFixture(t, s, request); day.FreeReservedBytes != 700 || day.FreeAvailableBytes() != 300 {
		t.Fatal("rejected seal changed shared availability", day)
	}
	if _, err := s.ApplyHourlyCreditProof(ctx, original.NodeID, original.AgentID, proof); err != nil {
		t.Fatal(err)
	}
	reserveHourlyFixture(t, s, other)
	day := readHourlyDayFixture(t, s, request)
	if day.FreeAdmittedBytes != 100 || day.FreeReservedBytes != 900 || day.FreeAvailableBytes() != 0 {
		t.Fatal("proof-based redistribution did not conserve the account pool", day)
	}
}
