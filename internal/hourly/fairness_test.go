package hourly

import "testing"

func TestAllocatorIndivisibleDatagramsRotateWithoutTwoNodeStarvation(t *testing.T) {
	for _, count := range []int{2, 3} {
		t.Run(string(rune('0'+count)), func(t *testing.T) {
			a, policy, peers, now := allocatorFixture(t, count)
			seen := map[string]bool{}
			for round := 0; round < 12; round++ {
				feedAllocatorDemands(t, a, now, policy.Owner, peers, MaxBurstBytes, round+1)
				actions := a.Tick(now, []Policy{policy})
				positive := 0
				for _, grant := range actions {
					if grant.RateBytesPerSecond > 0 || grant.BurstBytes > 0 {
						positive++
						seen[grant.Peer.NodeID] = true
						if grant.BurstBytes != MaxBurstBytes || grant.RateBytesPerSecond != policy.RateBytesPerSecond {
							t.Fatal("indivisible datagram received an unusable partial allocation")
						}
					}
				}
				if positive > 1 {
					t.Fatal("indivisible datagrams exceeded combined burst budget")
				}
				assertAllocatorBudget(t, a, now, policy)
				ackAllocatorGrants(t, a, now, actions)
				now = now.Add(DecisionInterval)
			}
			if len(seen) != count {
				t.Fatal("sticky shrink transition starved an eligible datagram peer")
			}
		})
	}
}

func TestAllocatorAtomicMinimumsAndIntegerRemaindersConserveBothBudgets(t *testing.T) {
	a, policy, peers, now := allocatorFixture(t, 3)
	policy.RateBytesPerSecond, policy.BurstBytes = 1001, 10
	for i, minimum := range []uint32{8, 8, 1} {
		feedAllocatorDemands(t, a, now, policy.Owner, peers[i:i+1], minimum, i+1)
	}
	grants := a.Tick(now, []Policy{policy})
	var rate, burst uint64
	for _, grant := range grants {
		rate += grant.RateBytesPerSecond
		burst += uint64(grant.BurstBytes)
		if grant.RateBytesPerSecond > 0 {
			minimum := uint32(1)
			if grant.Peer != peers[2] {
				minimum = 8
			}
			if grant.BurstBytes < minimum {
				t.Fatal("selected UDP peer cannot send its minimum datagram")
			}
		}
	}
	if rate != policy.RateBytesPerSecond || burst != uint64(policy.BurstBytes) {
		t.Fatal("integer splitting lost or minted account budget")
	}
	assertAllocatorBudget(t, a, now, policy)
}

func TestAllocatorRejectedShrinkKeepsEarlierPotentiallyLiveGrant(t *testing.T) {
	a, policy, peers, now := allocatorFixture(t, 1)
	feedAllocatorDemands(t, a, now, policy.Owner, peers, 1, 1)
	a.Tick(now, []Policy{policy})
	now = now.Add(DecisionInterval)
	feedAllocatorDemands(t, a, now, policy.Owner, peers, 1, 2)
	reduced := policy
	reduced.RateBytesPerSecond = 100
	shrink := a.Tick(now, []Policy{reduced})[0]
	if err := a.Acknowledge(now, Ack{Owner: shrink.Owner, Peer: shrink.Peer, Nonce: shrink.Nonce, Sequence: shrink.Sequence, Applied: false}); err != nil {
		t.Fatal(err)
	}
	if rate, _ := sumAllocatorReservations(a, now, policy.Owner.ID); rate != 1000 {
		t.Fatal("negative ACK reclaimed an older possibly-live grant")
	}
}

func TestAllocatorLateNegativeAckOfOldOfferCannotReleasePendingShrink(t *testing.T) {
	a, policy, peers, now := allocatorFixture(t, 2)
	feedAllocatorDemands(t, a, now, policy.Owner, peers[:1], 1, 1)
	old := a.Tick(now, []Policy{policy})[0] // It may have applied, but its success ACK was lost.
	now = now.Add(DecisionInterval)
	feedAllocatorDemands(t, a, now, policy.Owner, peers, 1, 2)
	// An unacknowledged plan stays sticky until realized or expired. An
	// authoritative policy decrease forces a new plan with a pending shrink.
	reduced := policy
	reduced.RateBytesPerSecond = 500
	actions := a.Tick(now, []Policy{reduced})
	var shrink Grant
	for _, grant := range actions {
		if grant.Peer == peers[0] {
			shrink = grant
		}
	}
	if shrink.RateBytesPerSecond != 250 {
		t.Fatal("fixture did not issue a pending shrink")
	}
	if err := a.Acknowledge(now, Ack{Owner: old.Owner, Peer: old.Peer, Nonce: old.Nonce, Sequence: old.Sequence, Applied: false}); err != nil {
		t.Fatal(err)
	}
	if rate, burst := sumAllocatorReservations(a, now, policy.Owner.ID); rate != 1000 || burst != uint64(MaxBurstBytes) {
		t.Fatal("negative old-sequence reply released undrained capacity")
	}
	now = now.Add(DecisionInterval)
	feedAllocatorDemands(t, a, now, policy.Owner, peers, 1, 3)
	for _, grant := range a.Tick(now, []Policy{reduced}) {
		if grant.Peer == peers[1] && (grant.RateBytesPerSecond > 0 || grant.BurstBytes > 0) {
			t.Fatal("another node increased while old capacity might still be draining")
		}
	}
}
