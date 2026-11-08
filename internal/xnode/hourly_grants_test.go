package xnode

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/hourly"
)

func senderHourlyGrant(now time.Time, sequence uint64) hourly.Grant {
	return hourly.Grant{Owner: hourly.Owner{ID: "owner", Generation: 1}, Peer: hourly.Peer{NodeID: "node", AgentID: masterHourlyAgent, BootID: masterHourlyBoot, SessionID: masterHourlySession}, Nonce: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Sequence: sequence, DayUTC: now.UTC().Format(time.DateOnly), Lifetime: 10 * time.Second, RateBytesPerSecond: 1000, BurstBytes: hourly.MaxBurstBytes, ConservativeUntil: now.Add(10 * time.Second)}
}

func TestHourlySenderAdmitsAndConsumesMoreThan1024DistinctOwners(t *testing.T) {
	now := time.Now()
	sender := newHourlyGrantSender()
	for i := 0; i < 1200; i++ {
		grant := senderHourlyGrant(now, uint64(i+1))
		grant.Owner.ID = fmt.Sprintf("owner-%04d", i)
		sender.offer(now, grant)
	}
	if len(sender.grants) != 1200 {
		t.Fatal("sender capacity silently excluded higher-sorted eligible owners")
	}
	seen := map[string]bool{}
	for range 1200 {
		grant, ok := sender.next(now)
		if !ok || seen[grant.Owner.ID] {
			t.Fatal("pending owner was starved or sent twice before the rest")
		}
		seen[grant.Owner.ID] = true
		sender.complete(grant)
	}
	if len(seen) != 1200 {
		t.Fatal("sender did not consume every pending owner")
	}
}

func TestHourlySenderCoalescesSequencesAndPreservesSingleWriter(t *testing.T) {
	now := time.Now()
	sender := newHourlyGrantSender()
	latest := senderHourlyGrant(now, 2)
	if !sender.offer(now, latest) {
		t.Fatal("first offer did not start the sender")
	}
	older := senderHourlyGrant(now, 1)
	if sender.offer(now, older) {
		t.Fatal("older offer started another writer")
	}
	grant, ok := sender.next(now)
	if !ok || grant.Sequence != 2 {
		t.Fatal("older queued offer replaced latest grant")
	}
	if sender.offer(now, latest) {
		t.Fatal("in-flight retry started another writer")
	}
	newer := senderHourlyGrant(now, 3)
	if sender.offer(now, newer) {
		t.Fatal("newer coalesced offer started another writer")
	}
	sender.complete(grant)
	grant, ok = sender.next(now)
	if !ok || grant.Sequence != 3 {
		t.Fatal("in-flight completion discarded a newer offer")
	}
	sender.complete(grant)
	if _, ok = sender.next(now.Add(11 * time.Second)); ok || len(sender.grants) != 0 {
		t.Fatal("expired unsent offers retained authority")
	}
}

func TestHourlySenderStaleSessionAndShortenedPolicyFailBeforeQueue(t *testing.T) {
	m, peer, repo, now := masterHourlyRateFixture(t)
	grant := senderHourlyGrant(*now, 1)
	replacement := &agentPeer{nodeID: peer.nodeID, agentID: peer.agentID, bootID: peer.bootID, hourlySession: "ffffffffffffffffffffffffffffffff", ctx: peer.ctx, cancel: peer.cancel}
	m.mu.Lock()
	m.peers[peer.nodeID] = replacement
	m.mu.Unlock()
	if err := m.sendHourlyGrant(peer, grant); !errors.Is(err, ErrUnavailable) {
		t.Fatal("old session reached the socket writer", err)
	}
	m.mu.Lock()
	m.peers[peer.nodeID] = peer
	m.mu.Unlock()
	repo.policies[0].NodePaidThrough[peer.nodeID] = now.Add(time.Second)
	m.refreshHourlyPolicies(peer.ctx)
	if err := m.sendHourlyGrant(peer, grant); !errors.Is(err, ErrUnavailable) {
		t.Fatal("shortened paid boundary changed an immutable same-sequence grant", err)
	}
}
