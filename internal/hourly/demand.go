package hourly

import (
	"encoding/hex"
	"strings"
	"time"
)

func validOwner(owner Owner) bool {
	return owner.ID != "" && len(owner.ID) <= 128 && owner.Generation <= 1<<63-1
}
func validStream(value string) bool {
	if len(value) != 32 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
func validPeer(peer Peer) bool {
	return peer.NodeID != "" && len(peer.NodeID) <= 128 && validStream(peer.AgentID) && validStream(peer.BootID) && validStream(peer.SessionID)
}

// Connect is called only for an authenticated, negotiated control connection.
// Replacing its session invalidates input, but never frees an old reservation.
func (a *Allocator) Connect(now time.Time, peer Peer) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.advance(now) {
		return ErrStale
	}
	if !validPeer(peer) {
		return ErrInvalid
	}
	if _, exists := a.peers[peer.NodeID]; !exists && len(a.peers) >= a.limits.MaxPeers {
		return ErrCapacity
	}
	a.peers[peer.NodeID] = peer
	for _, state := range a.accounts {
		for key, demand := range state.demands {
			if key.NodeID == peer.NodeID && demand.Peer != peer {
				delete(state.demands, key)
			}
		}
	}
	return nil
}

// Disconnect removes only current demand eligibility. Existing allocations stay.
func (a *Allocator) Disconnect(now time.Time, peer Peer) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.advance(now) || a.peers[peer.NodeID] != peer {
		return
	}
	delete(a.peers, peer.NodeID)
}

// Observe keeps the first receipt/deadline of a nonce. Repeated hints cannot
// refresh a lease, resurrect a superseded nonce or overtake ordered input.
func (a *Allocator) Observe(now time.Time, demand Demand) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.advance(now) {
		return ErrStale
	}
	if !validOwner(demand.Owner) || !validPeer(demand.Peer) || !validStream(demand.Nonce) || demand.Lifetime <= 0 || demand.Lifetime > MaxLifetime || demand.MinAtomicBytes > MaxBurstBytes || demand.PendingBytes > 1<<63-1 || demand.PendingBytes > 0 && demand.MinAtomicBytes == 0 || uint64(demand.MinAtomicBytes) > demand.PendingBytes {
		return ErrInvalid
	}
	if a.peers[demand.Peer.NodeID] != demand.Peer {
		return ErrStale
	}
	if demand.ReceivedAt.IsZero() || demand.ReceivedAt.After(now) {
		return ErrInvalid
	}
	if !demand.ReceivedAt.Add(demand.Lifetime).After(now) {
		return ErrExpired
	}
	state, err := a.account(demand.Owner)
	if err != nil {
		return err
	}
	if demand.Owner.Generation < state.generation {
		return ErrStale
	}
	key := demandKey{demand.Owner, demand.Peer.NodeID}
	if previous := state.demands[key]; previous != nil {
		if previous.Peer == demand.Peer && previous.Nonce == demand.Nonce {
			if !previous.ReceivedAt.Add(previous.Lifetime).After(now) {
				return ErrExpired
			}
			return nil
		}
		if !demand.ReceivedAt.After(previous.ReceivedAt) {
			return ErrStale
		}
	} else if len(state.demands) >= a.limits.MaxDemandsPerAccount {
		return ErrCapacity
	}
	nonce := nonceKey{demand.Owner, demand.Peer, demand.Nonce}
	if _, seen := state.seen[nonce]; seen {
		return ErrStale
	}
	if len(state.seen) >= a.limits.MaxNoncesPerAccount {
		return ErrCapacity
	}
	state.seen[nonce] = demand.ReceivedAt.Add(MaxLifetime)
	state.demands[key] = &demandState{Demand: demand}
	return nil
}

func (a *Allocator) active(now time.Time, state *account, key allocationKey) *demandState {
	demand := state.demands[demandKey{key.Owner, key.Peer.NodeID}]
	if demand == nil || demand.Peer != key.Peer || a.peers[key.Peer.NodeID] != key.Peer || !demand.ReceivedAt.Add(demand.Lifetime).After(now) {
		return nil
	}
	return demand
}
