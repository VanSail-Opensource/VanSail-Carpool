package hourly

import (
	"sort"
	"time"
)

// Tick accepts a complete current eligibility/policy view and returns actions at
// most once every two seconds. Account generations share the same held budget.
func (a *Allocator) Tick(now time.Time, policies []Policy) []Grant {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.advance(now) {
		return nil
	}
	a.applyPolicies(now, policies)
	if now.Before(a.quietUntil) || now.Before(a.nextDecision) {
		return nil
	}
	a.nextDecision = now.Add(DecisionInterval)
	var ids []string
	for id := range a.accounts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var actions []Grant
	for _, id := range ids {
		state := a.accounts[id]
		if !a.planValid(now, state) || planRealized(state) {
			state.plan = a.makePlan(now, state)
		}
		actions = append(actions, a.ownerTick(now, state)...)
	}
	return actions
}

func (a *Allocator) applyPolicies(now time.Time, policies []Policy) {
	accepted := map[string]Policy{}
	duplicates := map[string]bool{}
	for _, policy := range policies {
		if _, exists := accepted[policy.Owner.ID]; exists {
			duplicates[policy.Owner.ID] = true
		}
		accepted[policy.Owner.ID] = policy
	}
	for id, state := range a.accounts {
		state.policy = Policy{Owner: Owner{ID: id, Generation: state.generation}}
	}
	for id, policy := range accepted {
		if duplicates[id] || !validOwner(policy.Owner) || !policy.PaidThrough.After(now) || policy.RateBytesPerSecond > 1_000_000_000_000 || policy.BurstBytes > MaxBurstBytes {
			continue
		}
		state := a.accounts[id]
		if state == nil || policy.Owner.Generation < state.generation {
			continue
		}
		state.generation, state.policy = policy.Owner.Generation, policy
	}
}

func sortedAllocationKeys(values map[allocationKey]*allocation) []allocationKey {
	keys := make([]allocationKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return reservationLess(Reservation{Owner: keys[i].Owner, Peer: keys[i].Peer}, Reservation{Owner: keys[j].Owner, Peer: keys[j].Peer})
	})
	return keys
}

func (a *Allocator) ownerTick(now time.Time, state *account) []Grant {
	var result []Grant
	issued := map[allocationKey]bool{}
	shrinking := false
	for _, key := range sortedAllocationKeys(state.allocations) {
		lease := state.allocations[key]
		value, wanted := held(lease), state.plan.targets[key]
		if value.rate <= wanted.rate && value.burst <= wanted.burst {
			continue
		}
		shrinking = true
		var paid time.Time
		for _, record := range lease.records {
			if record.ConservativeUntil.After(paid) {
				paid = record.ConservativeUntil
			}
		}
		if key.Owner == state.policy.Owner && state.policy.PaidThrough.After(now) {
			paid = minTime(paid, state.policy.PaidThrough)
		}
		grant, ok := a.issue(now, state, key, target{rate: min(value.rate, wanted.rate), burst: min(value.burst, wanted.burst)}, paid)
		if ok {
			result = append(result, grant)
			issued[key] = true
		}
	}
	var demands []allocationKey
	for _, demand := range state.demands {
		key := allocationKey{demand.Owner, demand.Peer}
		if key.Owner == state.policy.Owner && state.policy.PaidThrough.After(now) && a.active(now, state, key) != nil {
			demands = append(demands, key)
		}
	}
	sort.Slice(demands, func(i, j int) bool { return demands[i].Peer.NodeID < demands[j].Peer.NodeID })
	for _, key := range demands {
		if issued[key] {
			continue
		}
		wanted, current := state.plan.targets[key], held(state.allocations[key])
		if wanted.rate < current.rate || wanted.burst < current.burst {
			continue
		}
		increase := wanted.rate > current.rate || wanted.burst > current.burst
		if increase && shrinking {
			wanted = current // Still permit separately reserved full-speed credit.
			increase = false
		}
		total := heldTotal(state)
		if increase && (total.rate-current.rate+wanted.rate > state.policy.RateBytesPerSecond || uint64(total.burst-current.burst)+uint64(wanted.burst) > uint64(state.policy.BurstBytes)) {
			wanted = current
		}
		if grant, ok := a.issue(now, state, key, wanted, state.policy.PaidThrough); ok {
			result = append(result, grant)
			issued[key] = true
		}
	}
	for _, key := range sortedAllocationKeys(state.allocations) {
		if issued[key] || a.peers[key.Peer.NodeID] != key.Peer {
			continue
		}
		lease := state.allocations[key]
		latest := lease.records[len(lease.records)-1]
		wanted := state.plan.targets[key]
		if !latest.acknowledged && !latest.rejected && latest.RateBytesPerSecond <= wanted.rate && latest.BurstBytes <= wanted.burst {
			result = append(result, latest.Grant)
		}
	}
	return result
}
