package xnode

import (
	"math"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// Confirmed SQL maxima are the only source of this disposable throughput
// estimate. It sizes reservations; the locked ledger alone grants availability.
func (m *Master) observeHourlyCreditProgress(peer *agentPeer, rows []model.HourlyCreditReservation) {
	now := m.service.now()
	c := m.hourlyCredits
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.peers[peer]
	if state == nil {
		return
	}
	frame := map[wire.HourlyOwner]float64{}
	for _, r := range rows {
		current := r.AdmittedUploadBytes + r.AdmittedDownloadBytes
		previous, exists := state.observed[r.ReservationID]
		if !exists || r.FinalProofDigest != "" {
			continue
		}
		if current > previous.bytes && now.After(previous.at) {
			owner := wire.HourlyOwner{ID: r.OwnerID, Generation: uint64(r.AccountGeneration)}
			frame[owner] += float64(current-previous.bytes) / now.Sub(previous.at).Seconds()
			state.observed[r.ReservationID] = hourlyCreditObserved{bytes: current, at: now}
			if _, exists := state.hints[owner]; exists || c.hintCount < maxHourlyCreditDemandHints {
				hint, exists := state.hints[owner]
				if !exists {
					c.hintCount++
				}
				hint.activeAt = now
				state.hints[owner] = hint
			}
		}
	}
	for owner, rate := range frame {
		state.rates[owner] = hourlyCreditRate{bytesPerSecond: rate, at: now}
	}
	for owner, rate := range state.rates {
		if now.Sub(rate.at) > time.Minute {
			delete(state.rates, owner)
		}
	}
}

func (m *Master) hourlyCreditTarget(peer *agentPeer, owner wire.HourlyOwner, unconsumed int64) int64 {
	now := m.service.now()
	c := m.hourlyCredits
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.peers[peer]
	if state == nil {
		return 1
	}
	target := hourlyCreditQuantum
	if rate, ok := state.rates[owner]; ok && !now.Before(rate.at) && now.Sub(rate.at) <= hourlyCreditDemandFreshness {
		coverage := time.Second + 2*min(state.roundTrip, 4*time.Second)
		proposal := math.Ceil(rate.bytesPerSecond * coverage.Seconds())
		// Clamp before float conversion; this never authorizes more than the
		// exact pool. Floating point is confined to the live sizing estimate.
		if proposal >= float64(unconsumed) {
			target = unconsumed
		} else if proposal > float64(target) {
			target = int64(proposal)
		}
	}
	demanding := 0
	for candidate, other := range c.peers {
		hint, ok := other.hints[owner]
		if ok && candidate.ctx.Err() == nil && !other.retireAll && !other.retire[owner] && hourlyCreditHintDemanding(hint, now) {
			demanding++
		}
	}
	return max(1, min(target, unconsumed/int64(max(1, demanding))))
}
