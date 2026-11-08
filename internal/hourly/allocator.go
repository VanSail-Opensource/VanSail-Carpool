package hourly

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrInvalid  = errors.New("invalid hourly allocation input")
	ErrStale    = errors.New("stale hourly allocation input")
	ErrExpired  = errors.New("expired hourly allocation input")
	ErrCapacity = errors.New("hourly allocator capacity exhausted")
)

type allocationKey struct {
	Owner Owner
	Peer  Peer
}
type demandKey struct {
	Owner  Owner
	NodeID string
}
type nonceKey struct {
	Owner Owner
	Peer  Peer
	Nonce string
}
type demandState struct {
	Demand
	issued bool
}
type record struct {
	Grant
	acknowledged bool
	rejected     bool
}
type allocation struct {
	records []record
	lastAck uint64
}
type plan struct {
	policy  Policy
	created time.Time
	targets map[allocationKey]target
}
type target struct {
	rate  uint64
	burst uint32
}
type account struct {
	demands              map[demandKey]*demandState
	seen                 map[nonceKey]time.Time
	allocations          map[allocationKey]*allocation
	policy               Policy
	generation, sequence uint64
	cursor               int
	plan                 *plan
}

// Allocator has no timers, persistence, callbacks or I/O. Returned actions must
// enter the existing transport queue after its mutex has been released.
type Allocator struct {
	mu                                sync.Mutex
	limits                            Limits
	quietUntil, lastNow, nextDecision time.Time
	accounts                          map[string]*account
	peers                             map[string]Peer
}

// New waits ten seconds before fresh grants so pre-restart agent leases expire.
func New(now time.Time, limits Limits) *Allocator {
	limits.MaxAccounts = bound(limits.MaxAccounts, 8192)
	limits.MaxPeers = bound(limits.MaxPeers, 4096)
	limits.MaxDemandsPerAccount = bound(limits.MaxDemandsPerAccount, 128)
	limits.MaxNoncesPerAccount = bound(limits.MaxNoncesPerAccount, 512)
	return &Allocator{limits: limits, quietUntil: now.Add(MaxLifetime), lastNow: now, accounts: map[string]*account{}, peers: map[string]Peer{}}
}

func bound(value, maximum int) int {
	if value <= 0 || value > maximum {
		return maximum
	}
	return value
}
func (a *Allocator) advance(now time.Time) bool {
	if now.IsZero() || now.Before(a.lastNow) {
		return false
	}
	if a.lastNow.IsZero() {
		a.quietUntil = now.Add(MaxLifetime)
	}
	a.lastNow = now
	a.prune(now)
	return true
}

func (a *Allocator) account(owner Owner) (*account, error) {
	if state := a.accounts[owner.ID]; state != nil {
		return state, nil
	}
	if len(a.accounts) >= a.limits.MaxAccounts {
		return nil, ErrCapacity
	}
	state := &account{demands: map[demandKey]*demandState{}, seen: map[nonceKey]time.Time{}, allocations: map[allocationKey]*allocation{}}
	a.accounts[owner.ID] = state
	return state, nil
}

func (a *Allocator) prune(now time.Time) {
	for id, state := range a.accounts {
		for key, expiry := range state.seen {
			if !expiry.After(now) {
				delete(state.seen, key)
			}
		}
		for key, demand := range state.demands {
			if !demand.ReceivedAt.Add(demand.Lifetime).After(now) && (key.Owner.Generation < state.generation || a.peers[key.NodeID] != demand.Peer) {
				delete(state.demands, key)
			}
		}
		for key, allocation := range state.allocations {
			live := allocation.records[:0]
			for _, grant := range allocation.records {
				if grant.ConservativeUntil.After(now) {
					live = append(live, grant)
				}
			}
			allocation.records = live
			if len(live) == 0 {
				delete(state.allocations, key)
			}
		}
		if len(state.allocations) == 0 && len(state.seen) == 0 && !state.policy.PaidThrough.Add(MaxLifetime).After(now) {
			delete(a.accounts, id)
		}
	}
}
