package database

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestHourlyOutstandingRequiresScopeAndIncludesHistoricalGenerations(t *testing.T) {
	s, request := hourlyCreditFixture(t)
	ctx := context.Background()
	old := reserveHourlyFixture(t, s, request)
	if _, err := s.OutstandingHourlyCredits(ctx, "", -1, "", 128); !errors.Is(err, ErrConflict) {
		t.Fatal("unscoped all-account query admitted", err)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE users SET account_generation=1 WHERE id=$1`, request.OwnerID); err != nil {
		t.Fatal(err)
	}
	current := request
	current.ReservationID, current.AccountGeneration = "new-generation", 1
	if _, err := s.EnsureHourlyCreditDay(ctx, current.OwnerID, 1, current.DayUTC, 1000); err != nil {
		t.Fatal(err)
	}
	reserveHourlyFixture(t, s, current)
	other := createTestUser(t, s, 89408)
	foreign := request
	foreign.ReservationID, foreign.OwnerID = "foreign-owner", other.ID
	if _, err := s.EnsureHourlyCreditDay(ctx, other.ID, 0, request.DayUTC, 1000); err != nil {
		t.Fatal(err)
	}
	reserveHourlyFixture(t, s, foreign)
	for _, scope := range []struct {
		owner string
		gen   int64
		node  string
		count int
	}{{request.OwnerID, -1, "", 2}, {request.OwnerID, 1, "", 1}, {"", -1, request.NodeID, 3}, {request.OwnerID, -1, "unknown-node", 0}} {
		rows, err := s.OutstandingHourlyCredits(ctx, scope.owner, scope.gen, scope.node, 128)
		if err != nil || len(rows) != scope.count {
			t.Fatal("scope leaked or lost original-generation evidence", scope, rows, err)
		}
	}
	proof := hourlyProofFixture(t, old, 0, 0, 1000, 0, false)
	if _, err := s.ApplyHourlyCreditProof(ctx, old.NodeID, old.AgentID, proof); err != nil {
		t.Fatal(err)
	}
	rows, err := s.OutstandingHourlyCredits(ctx, request.OwnerID, -1, "", 128)
	if err != nil || len(rows) != 1 || rows[0].ReservationID != current.ReservationID {
		t.Fatal("settlement crossed owner/generation scope", rows, err)
	}
	foreignDay := readHourlyDayFixture(t, s, foreign)
	if foreignDay.FreeReservedBytes != 1000 {
		t.Fatal("foreign obligation released", foreignDay)
	}
}

func TestHourlyOnlyNodeHistoryRetiresAfterProofAndKeepsTokenUntilThen(t *testing.T) {
	s, request := hourlyCreditFixture(t)
	ctx := context.Background()
	r := reserveHourlyFixture(t, s, request)
	token := bytes.Repeat([]byte{7}, 32)
	if _, err := s.DB().ExecContext(ctx, `UPDATE xnode_nodes SET token_hash=$2 WHERE id=$1`, r.NodeID, token); err != nil {
		t.Fatal(err)
	}
	version := func() int64 {
		v, _, err := s.XNodeCompileVersion(ctx, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if err := s.PrepareXNodeRemoval(ctx, r.NodeID, version()); err != nil {
		t.Fatal(err)
	}
	_, err := s.FinishXNodeRemoval(ctx, r.NodeID, version(), r.OwnerID, "hourly-pending")
	var policy *xnode.PolicyError
	if !errors.As(err, &policy) || policy.Code != "XNODE_REMOVAL_HOURLY_CREDIT_PENDING" {
		t.Fatal("unfinished credit did not block credential revocation", err)
	}
	node, err := s.XNodeNode(ctx, r.NodeID)
	if err != nil || !bytes.Equal(node.TokenHash, token) || node.RetiredAt != nil {
		t.Fatal("control token lost before proof", node, err)
	}
	proof := hourlyProofFixture(t, r, 0, 0, r.ByteCeiling, 0, false)
	if _, err = s.ApplyHourlyCreditProof(ctx, r.NodeID, r.AgentID, proof); err != nil {
		t.Fatal(err)
	}
	// Exclude all other history signals: the reservation itself must retain the
	// node tombstone, even when there is no current agent binding/native period.
	if _, err = s.DB().ExecContext(ctx, `UPDATE xnode_nodes SET agent_id=NULL WHERE id=$1`, r.NodeID); err != nil {
		t.Fatal(err)
	}
	retired, err := s.FinishXNodeRemoval(ctx, r.NodeID, version(), r.OwnerID, "hourly-final")
	if err != nil || !retired {
		t.Fatal("hourly-only history attempted physical deletion", retired, err)
	}
	node, err = s.XNodeNode(ctx, r.NodeID)
	if err != nil || node.RetiredAt == nil || len(node.TokenHash) != 0 {
		t.Fatal("retirement did not retain tombstone/revoke control", node, err)
	}
	stored, err := s.HourlyCreditReservation(ctx, r.ReservationID)
	if err != nil || stored.FinalProofDigest != proof.Digest || stored.UnusedBytes != 1000 {
		t.Fatal("retirement changed immutable credit evidence", stored, err)
	}
	request.ReservationID = "late"
	if _, err = s.ReserveHourlyCredit(ctx, request); !errors.Is(err, ErrConflict) {
		t.Fatal("retired node admitted a fresh obligation", err)
	}
}
