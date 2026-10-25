package xnode

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type ddnsChangeTestRepository struct {
	ddnsBindingTestRepository
	accepted []DDNSChangeInput
	failure  error
}

func (r *ddnsChangeTestRepository) XNodeCompileVersion(context.Context, time.Time) (int64, time.Time, error) {
	return 7, time.Time{}, nil
}
func (r *ddnsChangeTestRepository) XNodeDDNSChanges(context.Context) ([]DDNSChangeState, error) {
	return nil, nil
}
func (r *ddnsChangeTestRepository) AcceptXNodeDDNSChange(_ context.Context, in DDNSChangeInput, _ time.Time) (bool, error) {
	if r.failure != nil {
		return false, r.failure
	}
	r.accepted = append(r.accepted, in)
	return true, nil
}

func TestDDNSChangesRequireCurrentAuthenticatedConfiguration(t *testing.T) {
	ctx := context.Background()
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	repo := &ddnsChangeTestRepository{ddnsBindingTestRepository: ddnsBindingTestRepository{nodes: map[string]Node{}, bindings: map[string]DDNSHostBinding{}, role: "admin"}}
	s := NewService(repo, cacheTestSettings{}, vault, nil, nil)
	node := Node{ID: "node", AgentID: "agent", Name: "Node", ConfigID: "config", Multiplier: 1}
	settings := NodeSettings{DDNS: []wire.DDNSJob{{ID: "wan", Provider: wire.BuiltinDDNSProvider, IPv4: "auto", Interface: "eth0"}}, DDNSBindings: map[string]DDNSHostBinding{"wan": {IPv4HostIDs: []string{"host-v4"}}}}
	if _, err = s.SaveNodeAs(ctx, node, settings, "admin"); err != nil {
		t.Fatal(err)
	}
	_, settings, err = s.Node(ctx, node.ID)
	if err != nil {
		t.Fatal(err)
	}
	change := wire.DDNSChange{ID: "event", JobID: "wan", Generation: ddnsJobsForNode(node.ID, settings, nil)[0].Generation, Sequence: 1, IPv4: "203.0.113.8", ObservedAt: time.Now().UTC()}
	if err = s.acceptDDNSChange(ctx, node.ID, "foreign-agent", change); !errors.Is(err, ErrConflict) {
		t.Fatal("foreign agent accepted", err)
	}
	for _, kind := range []string{"generation", "job"} {
		stale := change
		if kind == "generation" {
			stale.Generation = "old-generation"
		} else {
			stale.JobID = "removed-job"
		}
		if err = s.acceptDDNSChange(ctx, node.ID, node.AgentID, stale); err != nil || len(repo.accepted) != 0 {
			t.Fatal("obsolete event mutated state", err)
		}
	}
	invalid := change
	invalid.IPv6 = "2001:db8::8"
	if err = s.acceptDDNSChange(ctx, node.ID, node.AgentID, invalid); !errors.Is(err, ErrInvalid) {
		t.Fatal("disabled family accepted", err)
	}
	invalid = change
	invalid.IPv4 = "::ffff:203.0.113.8"
	if err = s.acceptDDNSChange(ctx, node.ID, node.AgentID, invalid); !errors.Is(err, ErrInvalid) {
		t.Fatal("noncanonical address accepted", err)
	}
	repo.failure = ErrUnavailable
	if err = s.acceptDDNSChange(ctx, node.ID, node.AgentID, change); !errors.Is(err, ErrUnavailable) {
		t.Fatal("failed durable acceptance was hidden", err)
	}
	repo.failure = nil
	if err = s.acceptDDNSChange(ctx, node.ID, node.AgentID, change); err != nil || len(repo.accepted) != 1 {
		t.Fatal(err)
	}
	in := repo.accepted[0]
	if in.Operation.ActorUserID != "admin" || in.Operation.OwnerUserID != "" || in.Operation.Kind != DDNSOperationKind || in.Version != 7 || !slices.Equal(in.Binding.IPv4HostIDs, []string{"host-v4"}) {
		t.Fatal("wrong ownership or financial scope", in)
	}
	master := NewMaster(s, nil)
	message := wire.New("ddns.changes", "", 0, wire.DDNSChanges{Changes: []wire.DDNSChange{change}})
	if err = master.receiveDDNSChanges(ctx, &agentPeer{nodeID: node.ID, agentID: node.AgentID}, message); !errors.Is(err, ErrInvalid) {
		t.Fatal("unnegotiated event accepted", err)
	}
	peer := &agentPeer{nodeID: node.ID, agentID: node.AgentID, ddnsChanges: true}
	master.peers[node.ID] = peer
	tooMany := make([]wire.DDNSChange, wire.MaxDDNSChangesPerFrame+1)
	if err = master.receiveDDNSChanges(ctx, peer, wire.New("ddns.changes", "", 0, wire.DDNSChanges{Changes: tooMany})); !errors.Is(err, ErrInvalid) {
		t.Fatal("oversized event batch accepted", err)
	}
	oversized := message
	oversized.Data = make([]byte, (32<<10)+1)
	if err = master.receiveDDNSChanges(ctx, peer, oversized); !errors.Is(err, ErrInvalid) {
		t.Fatal("oversized event data accepted", err)
	}
	master.peers[node.ID] = &agentPeer{}
	if err = master.receiveDDNSChanges(ctx, peer, message); !errors.Is(err, ErrConflict) {
		t.Fatal("replaced peer accepted an event", err)
	}
}
