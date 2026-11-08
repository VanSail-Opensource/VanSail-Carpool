package xnode

import (
	"context"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (q *hourlyLiveLane) take() ([]hourlyInboundDemand, []wire.HourlyGrantAck) {
	q.mu.Lock()
	defer q.mu.Unlock()
	var demands []hourlyInboundDemand
	var acks []wire.HourlyGrantAck
	for range min(q.n, wire.MaxHourlyItems) {
		owner := q.order[q.head]
		q.order[q.head] = wire.HourlyOwner{}
		q.head, q.n = (q.head+1)%maxHourlyLiveOwners, q.n-1
		entry := q.items[owner]
		q.bytes -= hourlyLiveSize(owner, entry)
		if entry.demand != nil {
			demands = append(demands, *entry.demand)
			entry.demand = nil
		}
		if entry.applied != nil && len(acks) < wire.MaxHourlyItems {
			acks = append(acks, *entry.applied)
			entry.applied = nil
		}
		if entry.rejected != nil && len(acks) < wire.MaxHourlyItems {
			acks = append(acks, *entry.rejected)
			entry.rejected = nil
		}
		if entry.applied == nil && entry.rejected == nil {
			delete(q.items, owner)
		} else {
			q.order[(q.head+q.n)%maxHourlyLiveOwners] = owner
			q.n++
			q.bytes += hourlyLiveSize(owner, entry)
		}
	}
	return demands, acks
}

func (r *peerReceivers) runHourlyLive(ctx context.Context) {
	q := r.hourly.live
	for {
		if ctx.Err() != nil {
			return
		}
		demands, acks := q.take()
		if len(demands)+len(acks) == 0 {
			select {
			case <-ctx.Done():
				return
			case <-q.wake:
			}
			continue
		}
		if !r.master.currentHourlyPeer(r.peer) {
			return
		}
		r.master.acknowledgeHourlyGrants(r.peer, acks)
		r.master.observeHourlyDemands(r.peer, demands)
		var hints []wire.HourlyDemand
		now := r.master.service.now()
		for _, item := range demands {
			if !now.Before(item.latestAt) && now.Sub(item.latestAt) <= hourlyCreditDemandFreshness {
				hints = append(hints, item.demand)
			}
		}
		if len(hints) > 0 {
			r.master.requestHourlyCredit(r.peer, hints)
		}
	}
}
