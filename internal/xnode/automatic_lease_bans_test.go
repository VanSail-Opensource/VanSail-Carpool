package xnode

import (
	"context"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type automaticBanSpy struct {
	Repository
	calls int
}

func (s *automaticBanSpy) ReleaseAutomaticLeaseBans(context.Context, []AutomaticLeaseGrant, time.Time) error {
	s.calls++
	return nil
}

func TestAutomaticLeaseSteadyReportsDoNotWriteAndLateBanCommitRequeues(t *testing.T) {
	repo := &automaticBanSpy{}
	m := NewMaster(NewService(repo, nil, nil, nil, nil), nil)
	at := time.Now().UTC()
	items := []AutomaticLeaseObservation{{IP: "8.8.8.8", Node: "node", At: at}}
	m.automaticLeases.observe(items, at, time.Minute)
	if worked, err := m.releaseAutomaticLeaseBans(context.Background()); err != nil || !worked || repo.calls != 1 {
		t.Fatal("initial release was not batched", worked, repo.calls, err)
	}
	for range 10 {
		items[0].At = items[0].At.Add(time.Minute)
		m.automaticLeases.observe(items, items[0].At, time.Minute)
		if worked, err := m.releaseAutomaticLeaseBans(context.Background()); err != nil || worked || repo.calls != 1 {
			t.Fatal("steady observations performed lease database writes", worked, repo.calls, err)
		}
	}
	events := []wire.FirewallEvent{{Kind: "detected", IP: "8.8.8.8"}}
	if len(m.automaticLeases.protectedEventIPs(events, at, false)) != 1 {
		t.Fatal("current lease did not suppress a durable detection")
	}
	// Simulate a detection transaction committing after its earlier release
	// check: the post-commit pass must restore a release intent for this lease.
	m.automaticLeases.protectedEventIPs(events, at, true)
	if worked, err := m.releaseAutomaticLeaseBans(context.Background()); err != nil || !worked || repo.calls != 2 {
		t.Fatal("late durable ban could remain stranded", worked, repo.calls, err)
	}
}
