package xnode

import (
	"fmt"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func hourlyLaneDemand(i int) wire.HourlyDemand {
	return wire.HourlyDemand{Owner: wire.HourlyOwner{ID: fmt.Sprintf("owner-%05d", i), Generation: 1}, Nonce: masterHourlySession, LifetimeMillis: 10000, PendingBytes: 1, MinAtomicBytes: 1}
}

func TestHourlyLiveFullCapacityCoalescesDemandAndNativeDrainACKs(t *testing.T) {
	q := newHourlyLiveLane()
	at := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	for start := 0; start < maxHourlyLiveOwners; start += wire.MaxHourlyItems {
		var demands []wire.HourlyDemand
		var applied, rejected []wire.HourlyGrantAck
		for i := start; i < start+wire.MaxHourlyItems; i++ {
			demand := hourlyLaneDemand(i)
			demands = append(demands, demand)
			ack := wire.HourlyGrantAck{SessionID: masterHourlySession, Owner: demand.Owner, Nonce: demand.Nonce, Sequence: 1, Applied: true}
			applied = append(applied, ack)
			ack.Sequence, ack.Applied = 2, false
			rejected = append(rejected, ack)
		}
		q.offerDemands(demands, at)
		q.offerDemands(demands, at.Add(time.Second))
		if q.offerAcks(applied) != nil || q.offerAcks(rejected) != nil {
			t.Fatal("valid native ACK capture rejected")
		}
	}
	if q.n != maxHourlyLiveOwners || q.bytes > maxHourlyLiveBytes || q.dropped.Load() != 0 {
		t.Fatal("documented owner capacity did not fit bounded coalesced state", q.n, q.bytes)
	}
	seen, successes, failures := map[wire.HourlyOwner]bool{}, map[wire.HourlyOwner]bool{}, 0
	for q.n > 0 {
		demands, acks := q.take()
		if len(demands) > wire.MaxHourlyItems || len(acks) > wire.MaxHourlyItems {
			t.Fatal("consumer exceeded bounded allocator batch")
		}
		for _, item := range demands {
			if seen[item.demand.Owner] || !item.receivedAt.Equal(at) || !item.latestAt.Equal(at.Add(time.Second)) {
				t.Fatal("coalescing duplicated an owner or refreshed nonce authority")
			}
			seen[item.demand.Owner] = true
		}
		for _, ack := range acks {
			if ack.Applied {
				successes[ack.Owner] = true
			} else {
				if !successes[ack.Owner] {
					t.Fatal("higher negative ACK displaced the native drain proof")
				}
				failures++
			}
		}
	}
	if len(seen) != maxHourlyLiveOwners || len(successes) != maxHourlyLiveOwners || failures != maxHourlyLiveOwners || q.bytes != 0 {
		t.Fatal("full capture starved demand/native ACK evidence", len(seen), len(successes), failures, q.bytes)
	}
}

func TestHourlyLiveRingRotatesNewHotOwnersBehindWaitingOwners(t *testing.T) {
	q := newHourlyLiveLane()
	at := time.Now()
	for i := range 256 {
		q.offerDemands([]wire.HourlyDemand{hourlyLaneDemand(i)}, at)
	}
	first, _ := q.take()
	for _, item := range first {
		q.offerDemands([]wire.HourlyDemand{item.demand}, at)
	}
	next, _ := q.take()
	if len(next) != wire.MaxHourlyItems || next[0].demand.Owner != hourlyLaneDemand(128).Owner {
		t.Fatal("hot-owner retries displaced waiting owners")
	}
}

func TestHourlyLiveACKConflictAndNewNonceDoNotExtendOldReceipt(t *testing.T) {
	q := newHourlyLiveLane()
	at := time.Now()
	demand := hourlyLaneDemand(1)
	q.offerDemands([]wire.HourlyDemand{demand}, at)
	demand.Nonce = masterHourlyBoot
	q.offerDemands([]wire.HourlyDemand{demand}, at.Add(2*time.Second))
	old := demand
	old.Nonce = masterHourlySession
	q.offerDemands([]wire.HourlyDemand{old}, at.Add(time.Second))
	ack := wire.HourlyGrantAck{SessionID: masterHourlySession, Owner: demand.Owner, Nonce: demand.Nonce, Sequence: 4, Applied: true}
	if q.offerAcks([]wire.HourlyGrantAck{ack}) != nil {
		t.Fatal("valid ACK rejected")
	}
	ack.Applied, ack.Nonce = false, masterHourlySession
	if q.offerAcks([]wire.HourlyGrantAck{ack}) == nil {
		t.Fatal("same-sequence mismatched nonce replaced a native drain proof")
	}
	demands, acks := q.take()
	if len(demands) != 1 || demands[0].demand.Nonce != demand.Nonce || !demands[0].receivedAt.Equal(at.Add(2*time.Second)) || len(acks) != 1 || !acks[0].Applied {
		t.Fatal("reordered capture changed the latest nonce or successful ACK")
	}
}
