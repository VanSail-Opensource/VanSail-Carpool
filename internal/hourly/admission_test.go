package hourly

import (
	"testing"
	"time"
)

func TestAllocatorAdmissionIncludesIdlePaidOwnersAndRetiredHeldState(t *testing.T) {
	_, base, peers, now := allocatorFixture(t, 1)
	a := New(now.Add(-MaxLifetime), Limits{MaxAccounts: 2})
	if err := a.Connect(now, peers[0]); err != nil {
		t.Fatal(err)
	}
	first, second := base, base
	first.Owner.ID, second.Owner.ID = "paid-first", "paid-second"
	policies := []Policy{first, second}
	if !a.CanAdmit(now, first.Owner, policies) || a.CanAdmit(now, Owner{ID: "new", Generation: 1}, policies) || a.CanAdmit(now, Owner{}, policies) {
		t.Fatal("idle paid owners did not retain their future capacity")
	}
	retired := Demand{Owner: Owner{ID: "retired", Generation: 1}, Peer: peers[0], Nonce: "11111111111111111111111111111111", ReceivedAt: now, Lifetime: MaxLifetime, PendingBytes: 1, MinAtomicBytes: 1}
	if err := a.Observe(now, retired); err != nil {
		t.Fatal(err)
	}
	if a.CanAdmit(now, second.Owner, policies) {
		t.Fatal("retained retired state was excluded from the paid-owner union")
	}
	if a.CanAdmit(now.Add(MaxLifetime), Owner{ID: "new", Generation: 1}, []Policy{first}) || !a.lastNow.Equal(now) || len(a.accounts) != 1 {
		t.Fatal("capacity inspection advanced time or pruned old obligations")
	}
	a.Tick(now.Add(MaxLifetime), []Policy{first})
	if !a.CanAdmit(now.Add(MaxLifetime), second.Owner, []Policy{first}) {
		t.Fatal("normal allocator cleanup did not restore proven capacity")
	}
}

func TestAllocatorAdmissionHonorsColdStartRollbackAndExistingOwnerGeneration(t *testing.T) {
	a, policy, peers, now := allocatorFixture(t, 1)
	if a.CanAdmit(now.Add(-time.Millisecond), policy.Owner, nil) {
		t.Fatal("cold-start quiet permitted a new activation")
	}
	feedAllocatorDemands(t, a, now, policy.Owner, peers, 1, 1)
	policy.Owner.Generation = 2
	a.Tick(now, []Policy{policy})
	if a.CanAdmit(now, Owner{ID: policy.Owner.ID, Generation: 1}, []Policy{policy}) || a.CanAdmit(now.Add(-time.Millisecond), policy.Owner, []Policy{policy}) {
		t.Fatal("old generation or rollback passed admission readiness")
	}
	if !a.CanAdmit(now, policy.Owner, []Policy{policy}) {
		t.Fatal("current owner could not reuse its retained slot")
	}
}
