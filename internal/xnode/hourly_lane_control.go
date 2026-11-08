package xnode

import (
	"sync"
	"sync/atomic"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

const maxHourlyLiveOwners = 8192
const maxHourlyLiveBytes = 8 << 20

type hourlyInboundDemand struct {
	demand     wire.HourlyDemand
	receivedAt time.Time // First socket receipt for this nonce; never refreshed.
	latestAt   time.Time // Latest activity hint, independent of the rate deadline.
}

type hourlyLiveEntry struct {
	demand            *hourlyInboundDemand
	applied, rejected *wire.HourlyGrantAck
}

type hourlyLiveLane struct {
	mu      sync.Mutex
	items   map[wire.HourlyOwner]*hourlyLiveEntry
	order   [maxHourlyLiveOwners]wire.HourlyOwner
	head, n int
	bytes   int
	wake    chan struct{}
	dropped atomic.Uint64
}

func newHourlyLiveLane() *hourlyLiveLane {
	return &hourlyLiveLane{items: map[wire.HourlyOwner]*hourlyLiveEntry{}, wake: make(chan struct{}, 1)}
}

func hourlyLiveSize(owner wire.HourlyOwner, e *hourlyLiveEntry) int {
	if e == nil {
		return 0
	}
	size := 0
	if e.demand != nil {
		size += len(owner.ID) + len(e.demand.demand.Nonce) + 128
	}
	for _, ack := range []*wire.HourlyGrantAck{e.applied, e.rejected} {
		if ack != nil {
			size += len(owner.ID) + len(ack.SessionID) + len(ack.Nonce) + 128
		}
	}
	return size
}

func (q *hourlyLiveLane) put(owner wire.HourlyOwner, next *hourlyLiveEntry) {
	previous := q.items[owner]
	size := hourlyLiveSize(owner, next)
	if previous == nil && q.n >= maxHourlyLiveOwners || q.bytes-hourlyLiveSize(owner, previous)+size > maxHourlyLiveBytes {
		q.dropped.Add(1)
		return
	}
	if previous == nil {
		q.order[(q.head+q.n)%maxHourlyLiveOwners] = owner
		q.n++
	}
	q.bytes += size - hourlyLiveSize(owner, previous)
	q.items[owner] = next
}

func (q *hourlyLiveLane) offerDemands(items []wire.HourlyDemand, at time.Time) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, demand := range items {
		next := hourlyLiveEntry{}
		if previous := q.items[demand.Owner]; previous != nil {
			next = *previous
		}
		incoming := hourlyInboundDemand{demand: demand, receivedAt: at, latestAt: at}
		if previous := next.demand; previous != nil {
			if previous.demand.Nonce == demand.Nonce {
				incoming.receivedAt = previous.receivedAt
				incoming.demand.LifetimeMillis = min(incoming.demand.LifetimeMillis, previous.demand.LifetimeMillis)
			} else if !at.After(previous.receivedAt) {
				continue
			}
		}
		next.demand = &incoming
		q.put(demand.Owner, &next)
	}
	hourlyLaneSignal(q.wake)
}

func mergeHourlyAck(previous *wire.HourlyGrantAck, next wire.HourlyGrantAck) (*wire.HourlyGrantAck, error) {
	if previous != nil {
		if previous.Sequence == next.Sequence && previous.Nonce != next.Nonce {
			return nil, ErrInvalid
		}
		if previous.Sequence >= next.Sequence {
			return previous, nil
		}
	}
	next.Reason = "" // The allocator uses only exact scope, sequence and Applied.
	return &next, nil
}

func (q *hourlyLiveLane) offerAcks(items []wire.HourlyGrantAck) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, ack := range items {
		next := hourlyLiveEntry{}
		if previous := q.items[ack.Owner]; previous != nil {
			next = *previous
		}
		var err error
		if ack.Applied {
			next.applied, err = mergeHourlyAck(next.applied, ack)
		} else {
			next.rejected, err = mergeHourlyAck(next.rejected, ack)
		}
		if err != nil {
			return err
		}
		if next.applied != nil && next.rejected != nil {
			if next.applied.Sequence == next.rejected.Sequence && next.applied.Nonce != next.rejected.Nonce {
				return ErrInvalid
			}
			if next.applied.Sequence >= next.rejected.Sequence {
				next.rejected = nil
			}
		}
		q.put(ack.Owner, &next)
	}
	hourlyLaneSignal(q.wake)
	return nil
}

func (q *hourlyLiveLane) clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = map[wire.HourlyOwner]*hourlyLiveEntry{}
	q.order, q.head, q.n, q.bytes = [maxHourlyLiveOwners]wire.HourlyOwner{}, 0, 0, 0
}
