package hourly

import (
	"sort"
	"time"
)

func held(value *allocation) target {
	var result target
	if value != nil {
		for _, record := range value.records {
			result.rate = max(result.rate, record.RateBytesPerSecond)
			result.burst = max(result.burst, record.BurstBytes)
		}
	}
	return result
}

func heldTotal(state *account) target {
	var result target
	for _, allocation := range state.allocations {
		value := held(allocation)
		result.rate += value.rate
		result.burst += value.burst
	}
	return result
}

// Acknowledge releases earlier same-scope reservations only after an exact
// successful native drain acknowledgment. A negative response cannot establish
// that an earlier copy was never applied, so it releases no reservation.
func (a *Allocator) Acknowledge(now time.Time, ack Ack) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.advance(now) {
		return ErrStale
	}
	return a.acknowledge(now, ack)
}

// acknowledge runs after the caller advances time while holding a.mu.
func (a *Allocator) acknowledge(now time.Time, ack Ack) error {
	if !validOwner(ack.Owner) || !validPeer(ack.Peer) || !validStream(ack.Nonce) || ack.Sequence == 0 {
		return ErrInvalid
	}
	if a.peers[ack.Peer.NodeID] != ack.Peer {
		return ErrStale
	}
	state := a.accounts[ack.Owner.ID]
	if state == nil {
		return ErrStale
	}
	allocation := state.allocations[allocationKey{ack.Owner, ack.Peer}]
	if allocation == nil || ack.Sequence <= allocation.lastAck {
		return ErrStale
	}
	var matched *record
	for i := range allocation.records {
		entry := &allocation.records[i]
		if entry.Sequence == ack.Sequence && entry.Nonce == ack.Nonce {
			matched = entry
			break
		}
	}
	if matched == nil {
		return ErrStale
	}
	if !matched.ConservativeUntil.After(now) {
		return ErrExpired
	}
	if !ack.Applied {
		matched.rejected = true
		return nil
	}
	accepted := *matched
	accepted.acknowledged = true
	live := allocation.records[:0]
	for _, entry := range allocation.records {
		if entry.Sequence <= ack.Sequence {
			continue
		}
		live = append(live, entry)
	}
	live = append(live, accepted)
	sort.Slice(live, func(i, j int) bool { return live[i].Sequence < live[j].Sequence })
	allocation.records, allocation.lastAck = live, ack.Sequence
	return nil
}

// Snapshot returns conservative potentially-live reservations, never demands or
// financial usage. The returned values share no mutable allocator storage.
func (a *Allocator) Snapshot(now time.Time) []Reservation {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.advance(now) {
		now = a.lastNow
	}
	var result []Reservation
	for _, state := range a.accounts {
		for key, allocation := range state.allocations {
			value := held(allocation)
			reservation := Reservation{Owner: key.Owner, Peer: key.Peer, RateBytesPerSecond: value.rate, BurstBytes: value.burst}
			for _, record := range allocation.records {
				if record.ConservativeUntil.After(reservation.Until) {
					reservation.Until = record.ConservativeUntil
				}
			}
			result = append(result, reservation)
		}
	}
	sort.Slice(result, func(i, j int) bool { return reservationLess(result[i], result[j]) })
	return result
}

func reservationLess(left, right Reservation) bool {
	if left.Owner.ID != right.Owner.ID {
		return left.Owner.ID < right.Owner.ID
	}
	if left.Owner.Generation != right.Owner.Generation {
		return left.Owner.Generation < right.Owner.Generation
	}
	if left.Peer.NodeID != right.Peer.NodeID {
		return left.Peer.NodeID < right.Peer.NodeID
	}
	if left.Peer.SessionID != right.Peer.SessionID {
		return left.Peer.SessionID < right.Peer.SessionID
	}
	if left.Peer.AgentID != right.Peer.AgentID {
		return left.Peer.AgentID < right.Peer.AgentID
	}
	return left.Peer.BootID < right.Peer.BootID
}

func (a *Allocator) issue(now time.Time, state *account, key allocationKey, value target, paid time.Time) (Grant, bool) {
	demand := a.active(now, state, key)
	if demand == nil || demand.issued || state.sequence >= 1<<63-1 {
		return Grant{}, false
	}
	deadline := minTime(demand.ReceivedAt.Add(demand.Lifetime), paid)
	basis := demand.ReceivedAt.UTC()
	midnight := time.Date(basis.Year(), basis.Month(), basis.Day()+1, 0, 0, 0, 0, time.UTC)
	deadline = minTime(deadline, midnight)
	lifetime := deadline.Sub(demand.ReceivedAt).Truncate(time.Millisecond)
	deadline = demand.ReceivedAt.Add(lifetime)
	if lifetime <= 0 || !deadline.After(now) {
		return Grant{}, false
	}
	lease := state.allocations[key]
	if lease == nil {
		lease = &allocation{}
		state.allocations[key] = lease
	}
	if len(lease.records) >= 8 {
		return Grant{}, false
	}
	state.sequence++
	grant := Grant{Owner: key.Owner, Peer: key.Peer, Nonce: demand.Nonce, Sequence: state.sequence, DayUTC: now.UTC().Format(time.DateOnly), Lifetime: lifetime, RateBytesPerSecond: value.rate, BurstBytes: value.burst, ConservativeUntil: deadline}
	lease.records = append(lease.records, record{Grant: grant})
	demand.issued = true
	return grant, true
}

func minTime(left, right time.Time) time.Time {
	if left.Before(right) {
		return left
	}
	return right
}
