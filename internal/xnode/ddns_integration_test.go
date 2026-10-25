package xnode_test

import (
	"context"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestDDNSAcknowledgmentStagesBypassBeforeFreeRoutingRedistribution(t *testing.T) {
	f, peer, source, target := ddnsRoutingFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	server := httptest.NewServer(f.master)
	defer server.Close()
	a := newDDNSFixtureAgent(t, ctx, f, server, f.node.ID, fixtureAgent)
	b := newDDNSFixtureAgent(t, ctx, f, server, peer.ID, strings.Repeat("9", 32))
	for _, id := range []string{f.node.ID, peer.ID} {
		if err := f.master.WaitApplied(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	choices := []xnode.RoutingChoice{{ID: "path", InboundNodeID: f.node.ID, OutboundNodeID: peer.ID, InboundEndpointID: source.ID, OutboundEndpointID: target.ID, TemplateID: "search"}}
	if _, err := f.service.QueueOwnedRouting(ctx, f.user.ID, "ddns-initial", 0, choices); err != nil {
		t.Fatal(err)
	}
	waitDDNS(t, ctx, f, "initial route", func() bool {
		p, e := f.store.XNodeRoutingPlan(ctx, f.identity.ProxyUserID)
		return e == nil && p.Phase == "active"
	})
	before, err := f.store.XNodeRoutingPlan(ctx, f.identity.ProxyUserID)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := f.store.XNodeIdentity(ctx, f.identity.ProxyUserID)
	if err != nil {
		t.Fatal(err)
	}
	offline, err := f.service.SaveNode(ctx, xnode.Node{Name: "Offline", Enabled: true, Multiplier: 1, ConfigID: f.node.ConfigID}, xnode.NodeSettings{})
	if err != nil {
		t.Fatal(err)
	}
	var ledgerBefore int
	if err = f.store.DB().QueryRowContext(ctx, `SELECT count(*) FROM ledger_entries`).Scan(&ledgerBefore); err != nil {
		t.Fatal(err)
	}
	b.mu.Lock()
	jobs := slices.Clone(b.jobs)
	b.mu.Unlock()
	if len(jobs) != 1 || jobs[0].Generation == "" {
		t.Fatal("negotiated job missing", jobs)
	}
	change := wire.DDNSChange{ID: "address-change", JobID: jobs[0].ID, Generation: jobs[0].Generation, Sequence: 1, IPv4: "9.9.9.9", ObservedAt: time.Now().UTC()}
	for _, event := range []wire.DDNSChange{change, change} {
		if err = b.send(wire.New("ddns.changes", "event", 0, wire.DDNSChanges{Changes: []wire.DDNSChange{event}})); err != nil {
			t.Fatal(err)
		}
		select {
		case id := <-b.acks:
			if id != event.ID {
				t.Fatal(id)
			}
		case <-ctx.Done():
			t.Fatal("durable acknowledgment missing")
		}
	}
	states, err := f.store.XNodeDDNSChanges(ctx)
	if err != nil || len(states) != 1 || states[0].Phase != "staging" {
		t.Fatal(states, err)
	}
	var operations int
	if err = f.store.DB().QueryRowContext(ctx, `SELECT count(*) FROM provider_operations WHERE kind=?`, xnode.DDNSOperationKind).Scan(&operations); err != nil || operations != 1 {
		t.Fatal("duplicate charged/queued twice", operations, err)
	}
	a.mu.Lock()
	a.hold = true
	a.mu.Unlock()
	b.mu.Lock()
	b.hold = true
	b.mu.Unlock()
	waitDDNS(t, ctx, f, "staged firewall command", func() bool { a.mu.Lock(); defer a.mu.Unlock(); return len(a.pending) > 0 })
	hosts, err := f.service.Endpoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range hosts {
		if host.ID == target.ID && host.Address != "8.8.8.8" {
			t.Fatal("host activated before bypass acknowledgement", host)
		}
	}
	a.mu.Lock()
	if !slices.Contains(a.protected, "9.9.9.9") || !slices.Contains(a.protected, "8.8.8.8") {
		t.Fatal("new/old union missing", a.protected)
	}
	a.mu.Unlock()
	if err = a.releaseProtected(); err != nil {
		t.Fatal(err)
	}
	if err = b.releaseProtected(); err != nil {
		t.Fatal(err)
	}
	waitDDNS(t, ctx, f, "DDNS convergence", func() bool {
		states, e := f.store.XNodeDDNSChanges(ctx)
		if e != nil || len(states) != 1 || states[0].Phase != "settled" {
			return false
		}
		op, e := f.store.ProviderOperationByID(ctx, states[0].OperationID)
		return e == nil && op.Receipt.Status == "succeeded"
	})
	after, err := f.service.OwnedRoutingPlan(ctx, f.user.ID)
	if err != nil || after.Plan.Revision <= before.Revision || after.Plan.Phase != "active" || len(after.Plan.Choices) != 1 || after.Plan.Choices[0] != choices[0] {
		t.Fatal("routing choices were not retained", after.Plan, err)
	}
	compiled, err := f.service.Compile(ctx, f.node.ID)
	if err != nil || !strings.Contains(string(compiled.Xray), "9.9.9.9") {
		t.Fatal("route did not use updated host", err)
	}
	current, err := f.store.XNodeIdentity(ctx, f.identity.ProxyUserID)
	if err != nil || current.CredentialsCipher != identity.CredentialsCipher {
		t.Fatal("paid credentials changed", err)
	}
	var ledgerAfter int
	if err = f.store.DB().QueryRowContext(ctx, `SELECT count(*) FROM ledger_entries`).Scan(&ledgerAfter); err != nil || ledgerAfter != ledgerBefore {
		t.Fatal("automatic redistribution billed member", ledgerBefore, ledgerAfter, err)
	}
	f.master.NotifyFirewallChanged()
	waitDDNS(t, ctx, f, "old bypass removal", func() bool {
		a.mu.Lock()
		defer a.mu.Unlock()
		return slices.Contains(a.protected, "9.9.9.9") && !slices.Contains(a.protected, "8.8.8.8")
	})
	// An affected offline node retains desired routing and the previous bypass
	// until reconnect resumes the same operation and applies its barriers.
	change.ID = "offline-change"
	change.Sequence = 2
	change.IPv4 = "9.9.9.10"
	if err = b.send(wire.New("ddns.changes", "offline", 0, wire.DDNSChanges{Changes: []wire.DDNSChange{change}})); err != nil {
		t.Fatal(err)
	}
	select {
	case <-b.acks:
	case <-ctx.Done():
		t.Fatal("offline change acknowledgment missing")
	}
	_ = b.socket.Close()
	waitDDNS(t, ctx, f, "offline desired state", func() bool {
		states, e := f.store.XNodeDDNSChanges(ctx)
		return e == nil && len(states) == 1 && states[0].Phase == "routes"
	})
	a.mu.Lock()
	kept := slices.Contains(a.protected, "9.9.9.9")
	a.mu.Unlock()
	if !kept {
		t.Fatal("old bypass removed while route was offline")
	}
	b = newDDNSFixtureAgent(t, ctx, f, server, peer.ID, strings.Repeat("9", 32))
	waitDDNS(t, ctx, f, "affected-node reconnect", func() bool {
		states, e := f.store.XNodeDDNSChanges(ctx)
		return e == nil && len(states) == 1 && states[0].Phase == "settled"
	})
	// Offline unrelated nodes do not delay routes; they receive the latest bank.
	c := newDDNSFixtureAgent(t, ctx, f, server, offline.ID, strings.Repeat("8", 32))
	f.master.NotifyFirewallChanged()
	waitDDNS(t, ctx, f, "offline-node latest bypass", func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		return slices.Contains(c.protected, "9.9.9.10") && !slices.Contains(c.protected, "9.9.9.9") && !slices.Contains(c.protected, "8.8.8.8")
	})
}
