package xnode

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/hourly"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestHourlyBatchACKValidatesWholeBatchBeforeReleasingRate(t *testing.T) {
	m, peer, _, now := masterHourlyRateFixture(t)
	owner := wire.HourlyOwner{ID: "owner", Generation: 1}
	demand := wire.HourlyDemand{Owner: owner, Nonce: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", LifetimeMillis: 10000, PendingBytes: 1, MinAtomicBytes: 1}
	m.observeHourlyDemand(peer, demand, *now)
	policy := hourly.Policy{Owner: hourly.Owner{ID: owner.ID, Generation: owner.Generation}, PaidThrough: now.Add(time.Hour), RateBytesPerSecond: 1000, BurstBytes: hourly.MaxBurstBytes}
	m.hourly.allocator.Tick(*now, []hourly.Policy{policy})
	*now = now.Add(hourly.DecisionInterval)
	demand.Nonce = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	m.observeHourlyDemand(peer, demand, *now)
	policy.RateBytesPerSecond = 100
	shrink := m.hourly.allocator.Tick(*now, []hourly.Policy{policy})[0]
	ack := wire.HourlyGrantAck{SessionID: masterHourlySession, Owner: owner, Nonce: shrink.Nonce, Sequence: shrink.Sequence, Applied: true}
	batch := wire.HourlyGrantAckBatch{SessionID: masterHourlySession, Acks: []wire.HourlyGrantAck{ack, ack}}
	if err := m.receiveHourly(context.Background(), peer, wire.New("hourly.grant.acks", "", 0, batch), *now); err == nil {
		t.Fatal("duplicate-owner ACK batch accepted")
	}
	if reserved := m.hourly.allocator.Snapshot(*now); len(reserved) != 1 || reserved[0].RateBytesPerSecond != 1000 {
		t.Fatal("invalid batch partially released old capacity")
	}
	batch.Acks = batch.Acks[:1]
	if err := m.receiveHourly(context.Background(), peer, wire.New("hourly.grant.acks", "", 0, batch), *now); err != nil {
		t.Fatal(err)
	}
	if reserved := m.hourly.allocator.Snapshot(*now); len(reserved) != 1 || reserved[0].RateBytesPerSecond != 100 {
		t.Fatal("exact batched native drain did not release old capacity")
	}
}

func TestHourlyACKAndDemandRejectForeignSessionAndNonparticipatingNode(t *testing.T) {
	m, peer, _, now := masterHourlyRateFixture(t)
	foreign := "ffffffffffffffffffffffffffffffff"
	ack := wire.HourlyGrantAck{SessionID: foreign, Owner: wire.HourlyOwner{ID: "owner", Generation: 1}, Nonce: foreign, Sequence: 1, Applied: true}
	if err := m.receiveHourly(context.Background(), peer, wire.New("hourly.grant.ack", "", 0, ack), *now); err == nil {
		t.Fatal("foreign-session ACK accepted")
	}
	demand := wire.HourlyDemand{Owner: ack.Owner, Nonce: foreign, LifetimeMillis: 10000, PendingBytes: 1, MinAtomicBytes: 1}
	batch := wire.HourlyDemandBatch{SessionID: foreign, Demands: []wire.HourlyDemand{demand}}
	if err := m.receiveHourly(context.Background(), peer, wire.New("hourly.demand", "", 0, batch), *now); err == nil {
		t.Fatal("foreign-session demand accepted")
	}
	batch.SessionID = masterHourlySession
	batch.Demands[0].Owner.ID = "closed-owner"
	if err := m.receiveHourly(context.Background(), peer, wire.New("hourly.demand", "", 0, batch), *now); err != nil {
		t.Fatal("valid closed-owner hint should be ignored safely", err)
	}
	if grants := m.hourly.allocator.Tick(*now, m.hourly.allocatorPolicies(*now)); len(grants) != 0 {
		t.Fatal("closed-owner hint acquired rate allocation")
	}
}
