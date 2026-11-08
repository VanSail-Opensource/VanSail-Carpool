package xnode

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type hourlyExtraSpy struct{ requests []HourlyExtraRequest }

func (s *hourlyExtraSpy) BuyHourlyExtra(_ context.Context, request HourlyExtraRequest) error {
	s.requests = append(s.requests, request)
	return nil
}

func TestHourlyExtraBuyerRequiresOriginalFreshPositiveDemandAndNoHeldPool(t *testing.T) {
	m, peer, _, now := masterHourlyRateFixture(t)
	spy := &hourlyExtraSpy{}
	m.service.SetHourlyExtraBuyer(spy) // This fixture has no running workers.
	owner := wire.HourlyOwner{ID: "owner", Generation: 1}
	day := model.HourlyCreditDay{OwnerID: owner.ID, AccountGeneration: 1, DayUTC: now.UTC().Format(time.DateOnly), FreeAllowanceBytes: 100, FreeAdmittedBytes: 100}
	original := now.Add(-3 * time.Second)
	fresh := hourlyCreditHint{demand: wire.HourlyDemand{Owner: owner, PendingBytes: 17}, seenAt: original, activeAt: *now}
	for _, entry := range []struct {
		name string
		hint hourlyCreditHint
		day  model.HourlyCreditDay
		buy  bool
	}{
		{"active-only", hourlyCreditHint{activeAt: *now}, day, false},
		{"zero-hint", hourlyCreditHint{seenAt: *now, activeAt: *now}, day, false},
		{"stale-pending", hourlyCreditHint{demand: fresh.demand, seenAt: now.Add(-hourlyCreditDemandFreshness - time.Nanosecond), activeAt: *now}, day, false},
		{"held-free", fresh, model.HourlyCreditDay{FreeAllowanceBytes: 100, FreeAdmittedBytes: 99, FreeReservedBytes: 1}, false},
		{"held-extra", fresh, model.HourlyCreditDay{FreeAllowanceBytes: 100, FreeAdmittedBytes: 100, ExtraFundedBytes: 1, ExtraReservedBytes: 1}, false},
		{"available-free", fresh, model.HourlyCreditDay{FreeAllowanceBytes: 100, FreeAdmittedBytes: 99}, false},
		{"available-extra", fresh, model.HourlyCreditDay{FreeAllowanceBytes: 100, FreeAdmittedBytes: 100, ExtraFundedBytes: 1}, false},
		{"exhausted-positive", fresh, day, true},
	} {
		t.Run(entry.name, func(t *testing.T) {
			m.hourlyCredits.mu.Lock()
			state := m.hourlyCredits.stateLocked(peer, *now)
			state.hints[owner] = entry.hint
			m.hourlyCredits.mu.Unlock()
			before := len(spy.requests)
			attempted, err := m.buyDemandedHourlyExtra(context.Background(), peer, owner, entry.day)
			if err != nil || attempted != entry.buy || (len(spy.requests) != before) != entry.buy {
				t.Fatal("incorrect extra funding admission", attempted, spy.requests, err)
			}
		})
	}
	if len(spy.requests) != 1 {
		t.Fatal("fresh pending demand did not produce exactly one buyer call", spy.requests)
	}
	request := spy.requests[0]
	if request.OwnerID != owner.ID || request.AccountGeneration != 1 || request.NodeID != peer.nodeID || request.PendingBytes != 17 || !request.PendingObservedAt.Equal(original) || request.DayUTC != day.DayUTC {
		t.Fatal("buyer received client pricing or a refreshed pending timestamp", request)
	}
	m.mu.Lock()
	delete(m.peers, peer.nodeID)
	m.mu.Unlock()
	if attempted, err := m.buyDemandedHourlyExtra(context.Background(), peer, owner, day); err != nil || attempted {
		t.Fatal("disconnected peer bought extra traffic", attempted, err)
	}
}
