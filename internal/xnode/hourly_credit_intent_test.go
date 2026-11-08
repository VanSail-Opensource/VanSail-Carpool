package xnode

import (
	"context"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type hourlyIntentRepository struct {
	Repository
	hourlyCreditProductionRepository
	closed    bool
	resolved  []model.HourlyCreditRequest
	attempted []model.HourlyCreditRequest
	quotaWait bool
}

func (r *hourlyIntentRepository) ResolveHourlyCreditIntent(_ context.Context, request model.HourlyCreditRequest) (model.HourlyCreditReservation, error) {
	r.resolved = append(r.resolved, request)
	if r.closed {
		return model.HourlyCreditReservation{}, model.ErrHourlyCreditNotFound
	}
	return model.HourlyCreditReservation{}, model.ErrHourlyCreditUnavailable
}

func (r *hourlyIntentRepository) ReserveHourlyCredit(_ context.Context, request model.HourlyCreditRequest) (model.HourlyCreditReservation, error) {
	r.attempted = append(r.attempted, request)
	if r.quotaWait {
		return model.HourlyCreditReservation{}, model.ErrHourlyCreditUnavailable
	}
	return model.HourlyCreditReservation{}, ErrUnavailable
}

func TestHourlyUnfundableIntentsRotateWithoutConsumingMutationBudget(t *testing.T) {
	m, peer, _, now := masterHourlyRateFixture(t)
	r := &hourlyIntentRepository{quotaWait: true}
	m.service.repo = r
	c := m.hourlyCredits
	c.mu.Lock()
	state := c.stateLocked(peer, *now)
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		owner := wire.HourlyOwner{ID: id, Generation: 1}
		state.intents[owner] = model.HourlyCreditRequest{ReservationID: id, OwnerID: id, ByteCeiling: 1000}
	}
	c.mu.Unlock()
	for range 2 {
		changes, err := m.resolveHourlyCreditIntents(context.Background(), peer, r)
		if err != nil || changes != 0 {
			t.Fatal("expected quota wait blocked unrelated pending/owner work", changes, err)
		}
	}
	seen := map[string]bool{}
	for _, request := range r.attempted {
		seen[request.ReservationID] = true
	}
	if len(seen) != 5 {
		t.Fatal("fixed first-four intent selection starved another owner", seen)
	}
}

func TestRetiredHourlyIntentResolvesWithoutHintEligibilityOrNewReservation(t *testing.T) {
	m, peer, _, now := masterHourlyRateFixture(t)
	r := &hourlyIntentRepository{closed: true}
	m.service.repo = r
	owner := wire.HourlyOwner{ID: "owner", Generation: 1}
	request := model.HourlyCreditRequest{ReservationID: "uncertain-original", OwnerID: owner.ID, AccountGeneration: 1, DayUTC: "2026-11-09", ByteCeiling: 17, Kind: "extra"}
	c := m.hourlyCredits
	c.mu.Lock()
	state := c.stateLocked(peer, *now)
	state.retireAll = true
	state.intents[owner] = request
	c.mu.Unlock()
	count, err := m.resolveHourlyCreditIntents(context.Background(), peer, r)
	if err != nil || count != 1 || len(r.resolved) != 1 || r.resolved[0] != request || len(r.attempted) != 0 {
		t.Fatal("retired/ineligible intent did not use the exact admission fence", count, err)
	}
	c.mu.Lock()
	remaining := len(state.intents)
	c.mu.Unlock()
	if remaining != 0 {
		t.Fatal("definitively absent retired intent blocks removal forever")
	}
}

func TestActiveUncertainHourlyIntentRetriesExactTupleBeforeSizing(t *testing.T) {
	m, peer, _, now := masterHourlyRateFixture(t)
	r := &hourlyIntentRepository{}
	m.service.repo = r
	owner := wire.HourlyOwner{ID: "owner", Generation: 1}
	request := model.HourlyCreditRequest{ReservationID: "original", OwnerID: owner.ID, AccountGeneration: 1, DayUTC: "2026-11-09", ByteCeiling: 17, Kind: "extra"}
	c := m.hourlyCredits
	c.mu.Lock()
	c.stateLocked(peer, *now).intents[owner] = request
	c.mu.Unlock()
	if err := m.resolveHourlyCreditIntent(context.Background(), peer, r, owner, request); err == nil || len(r.attempted) != 1 || r.attempted[0] != request {
		t.Fatal("uncertain replay changed day/kind/amount or minted a new ID", r.attempted, err)
	}
	c.mu.Lock()
	retained := c.peers[peer].intents[owner]
	c.mu.Unlock()
	if retained != request {
		t.Fatal("uncertain retry discarded the original identity")
	}
}

func TestHourlyReactivationClearsOnlyExactTransientOwnerStop(t *testing.T) {
	m, peer, _, now := masterHourlyRateFixture(t)
	owner := wire.HourlyOwner{ID: "owner", Generation: 1}
	other := wire.HourlyOwner{ID: "owner", Generation: 0}
	c := m.hourlyCredits
	c.mu.Lock()
	state := c.stateLocked(peer, *now)
	state.retire[owner], state.retire[other], state.retireAll = true, true, true
	c.mu.Unlock()
	if err := m.ResumeHourlyCreditAdmission(owner.ID, int64(owner.Generation)); err != nil {
		t.Fatal(err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if state.retire[owner] || !state.retire[other] || !state.retireAll {
		t.Fatal("reactivation changed another generation or permanent node fence")
	}
}
