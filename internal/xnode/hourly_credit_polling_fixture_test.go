package xnode

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type hourlyPollingRepository struct {
	Repository
	hourlyCreditProductionRepository
	rows              []model.HourlyCreditReservation
	scans, held, days int
	onScan            func()
	heldBytes         int64
	emptyDay          bool
}

func (r *hourlyPollingRepository) UnfinalizedHourlyCredits(_ context.Context, _, _ string, after int64, limit int) ([]model.HourlyCreditReservation, error) {
	r.scans++
	if r.onScan != nil {
		r.onScan()
	}
	var rows []model.HourlyCreditReservation
	for _, row := range r.rows {
		if row.IssueSequence > after && len(rows) < limit {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func (r *hourlyPollingRepository) HourlyCreditHeldBytes(context.Context, model.HourlyCreditRequest) (int64, error) {
	r.held++
	return r.heldBytes, nil
}

func (r *hourlyPollingRepository) EnsureHourlyCreditDay(_ context.Context, owner string, generation int64, day string, allowance int64) (model.HourlyCreditDay, error) {
	r.days++
	if r.emptyDay {
		return model.HourlyCreditDay{}, nil
	}
	return model.HourlyCreditDay{OwnerID: owner, AccountGeneration: generation, DayUTC: day, FreeAllowanceBytes: allowance, FreeReservedBytes: hourlyCreditQuantum}, nil
}

func hourlyPollingFixture(t *testing.T, owners int) (*Master, *agentPeer, *hourlyPollingRepository, *time.Time) {
	t.Helper()
	m, peer, policies, now := masterHourlyRateFixture(t)
	c := m.hourlyCredits
	state := c.stateLocked(peer, *now)
	state.work = true
	for n := 0; n < owners; n++ {
		id := "owner"
		if n > 0 {
			id = fmt.Sprintf("owner-%d", n)
			policy := policies.policies[0]
			policy.OwnerID = id
			policies.policies = append(policies.policies, policy)
		}
		owner := wire.HourlyOwner{ID: id, Generation: 1}
		state.hints[owner] = hourlyCreditHint{demand: wire.HourlyDemand{Owner: owner, PendingBytes: 1}, seenAt: *now, activeAt: *now}
		c.hintCount++
	}
	m.refreshHourlyPolicies(context.Background())
	r := &hourlyPollingRepository{heldBytes: hourlyCreditQuantum}
	m.service.repo = r
	row := model.HourlyCreditReservation{HourlyCreditRequest: model.HourlyCreditRequest{
		ReservationID: "delivered", OwnerID: "owner", AccountGeneration: 1, DayUTC: now.UTC().Format(time.DateOnly),
		NodeID: peer.nodeID, AgentID: peer.agentID, BootID: peer.bootID, SessionID: peer.hourlySession, ByteCeiling: hourlyCreditQuantum,
	}, IssueSequence: 1}
	r.rows = []model.HourlyCreditReservation{row}
	state.delivered[row.ReservationID] = row.IssueSequence
	state.observed[row.ReservationID] = hourlyCreditObserved{at: *now}
	return m, peer, r, now
}

func runHourlyPollingPass(t *testing.T, m *Master, peer *agentPeer, now time.Time) {
	t.Helper()
	if peers := m.hourlyCredits.ready(now); len(peers) != 1 || peers[0] != peer {
		t.Fatal("expected immediate credit work", peers)
	}
	if err := m.produceHourlyCredits(peer); err != nil {
		t.Fatal(err)
	}
	m.hourlyCredits.finished(peer, false, false, now)
}
