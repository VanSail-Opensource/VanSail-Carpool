package hourly

import (
	"sort"
	"time"
)

func samePolicy(left, right Policy) bool {
	return left.Owner == right.Owner && left.RateBytesPerSecond == right.RateBytesPerSecond && left.BurstBytes == right.BurstBytes && left.PaidThrough.Equal(right.PaidThrough)
}

func (a *Allocator) planValid(now time.Time, state *account) bool {
	p := state.plan
	if p == nil || !samePolicy(p.policy, state.policy) || !p.created.Add(MaxLifetime).After(now) {
		return false
	}
	for key, value := range p.targets {
		demand := a.active(now, state, key)
		if demand == nil || max(uint32(1), demand.MinAtomicBytes) > value.burst {
			return false
		}
	}
	return true
}

func planRealized(state *account) bool {
	for key, allocation := range state.allocations {
		value, wanted := held(allocation), state.plan.targets[key]
		if value.rate > wanted.rate || value.burst > wanted.burst {
			return false
		}
	}
	for key, wanted := range state.plan.targets {
		confirmed := false
		if allocation := state.allocations[key]; allocation != nil {
			for _, record := range allocation.records {
				if record.acknowledged && record.RateBytesPerSecond == wanted.rate && record.BurstBytes == wanted.burst {
					confirmed = true
				}
			}
		}
		if !confirmed {
			return false
		}
	}
	return true
}

func (a *Allocator) makePlan(now time.Time, state *account) *plan {
	p := &plan{policy: state.policy, created: now, targets: map[allocationKey]target{}}
	var keys []allocationKey
	for _, demand := range state.demands {
		key := allocationKey{demand.Owner, demand.Peer}
		if demand.Owner == state.policy.Owner && a.active(now, state, key) != nil {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Peer.NodeID < keys[j].Peer.NodeID })
	if len(keys) == 0 || state.policy.RateBytesPerSecond == 0 || state.policy.BurstBytes == 0 {
		return p
	}
	start := state.cursor % len(keys)
	state.cursor = (start + 1) % len(keys)
	remaining := state.policy.BurstBytes
	var selected []allocationKey
	for offset := 0; offset < len(keys); offset++ {
		key := keys[(start+offset)%len(keys)]
		minimum := max(uint32(1), a.active(now, state, key).MinAtomicBytes)
		if minimum > remaining || uint64(len(selected)) >= state.policy.RateBytesPerSecond {
			continue
		}
		p.targets[key] = target{burst: minimum}
		remaining -= minimum
		selected = append(selected, key)
	}
	if len(selected) == 0 {
		return p
	}
	n := uint64(len(selected))
	for i, key := range selected {
		value := p.targets[key]
		value.rate = state.policy.RateBytesPerSecond / n
		if uint64(i) < state.policy.RateBytesPerSecond%n {
			value.rate++
		}
		value.burst += remaining / uint32(n)
		if uint32(i) < remaining%uint32(n) {
			value.burst++
		}
		p.targets[key] = value
	}
	return p
}
