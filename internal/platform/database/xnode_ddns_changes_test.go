package database

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func ddnsChangeFixture(t *testing.T) (*Store, xnode.DDNSChangeInput) {
	t.Helper()
	s := newTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, 48981)
	if err := s.SaveXNodeConfiguration(ctx, xnode.Configuration{ID: "ddns-config", Name: "Config", DocumentCipher: "encrypted"}); err != nil {
		t.Fatal(err)
	}
	node := xnode.Node{ID: "ddns-node", Name: "Node", ConfigID: "ddns-config", SettingsCipher: "discovery", MultiplierFP: 1_000_000}
	binding := xnode.DDNSHostBinding{ConfiguredBy: owner.ID}
	if err := s.SaveXNodeNodeDDNS(ctx, node, map[string]xnode.DDNSHostBinding{"wan": binding}); err != nil {
		t.Fatal(err)
	}
	if err := s.BindXNodeAgent(ctx, node.ID, "agent-one"); err != nil {
		t.Fatal(err)
	}
	node.AgentID = "agent-one"
	in := xnode.DDNSChangeInput{Node: node, Binding: binding, Change: wire.DDNSChange{ID: "event-one", JobID: "wan", Generation: "generation", Sequence: math.MaxUint64, IPv4: "203.0.113.8", ObservedAt: time.Now().UTC()}, Operation: providerops.CreateInput{ActorUserID: owner.ID, Kind: xnode.DDNSOperationKind, IdempotencyKey: "event-one", RequestFingerprint: strings.Repeat("a", 64), Items: []providerops.ItemInput{{Key: "ddns", TargetType: "xnode_ddns", TargetID: node.ID}}}}
	version, _, err := s.XNodeCompileVersion(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	in.Version = version
	return s, in
}

func TestDDNSChangesCommitOrderingAndOperationTogether(t *testing.T) {
	s, in := ddnsChangeFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	accepted, err := s.AcceptXNodeDDNSChange(ctx, in, now)
	if err != nil || !accepted {
		t.Fatal(accepted, err)
	}
	for _, sequence := range []uint64{math.MaxUint64, math.MaxUint64 - 1} {
		replay := in
		replay.Change.Sequence = sequence
		if fresh, err := s.AcceptXNodeDDNSChange(ctx, replay, now); err != nil || fresh {
			t.Fatal("duplicate/stale mutation", fresh, err)
		}
	}
	states, err := s.XNodeDDNSChanges(ctx)
	if err != nil || len(states) != 1 || states[0].Change.Sequence != math.MaxUint64 || states[0].Phase != "staging" || states[0].OperationID == "" {
		t.Fatal(states, err)
	}
	var operations, jobs int
	if err = s.DB().QueryRowContext(ctx, `SELECT count(*) FROM provider_operations WHERE kind=?`, xnode.DDNSOperationKind).Scan(&operations); err != nil {
		t.Fatal(err)
	}
	if err = s.DB().QueryRowContext(ctx, `SELECT count(*) FROM outbox_jobs WHERE kind='provider_operation'`).Scan(&jobs); err != nil || operations != 1 || jobs != 1 {
		t.Fatal(operations, jobs, err)
	}
	// Configuration changes and re-enrollment fence events validated earlier.
	for _, kind := range []string{"cipher", "revision", "agent"} {
		stale := in
		switch kind {
		case "cipher":
			stale.Node.SettingsCipher = "old"
		case "revision":
			stale.Version--
		case "agent":
			stale.Node.AgentID = "old-agent"
		}
		if _, err = s.AcceptXNodeDDNSChange(ctx, stale, now); err == nil {
			t.Fatal("stale fence accepted", kind)
		}
	}
	// Simulate an explicitly authorized identity reset. Ordinary enrollment
	// rotation does not permit a different agent to replace this identity.
	if _, err = s.DB().ExecContext(ctx, `UPDATE xnode_nodes SET agent_id=NULL WHERE id=?`, in.Node.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.BindXNodeAgent(ctx, in.Node.ID, "agent-two"); err != nil {
		t.Fatal(err)
	}
	in.Node.AgentID = "agent-two"
	in.Change.Sequence = 1
	in.Change.ID = "event-two"
	in.Change.IPv4 = "203.0.113.9"
	in.Operation.IdempotencyKey = "event-two"
	if accepted, err = s.AcceptXNodeDDNSChange(ctx, in, now); err != nil || !accepted {
		t.Fatal("new authenticated journal could not reset sequence", accepted, err)
	}
	states, err = s.XNodeDDNSChanges(ctx)
	if err != nil || len(states[0].PreviousIPs) != 1 || states[0].PreviousIPs[0] != "203.0.113.8" {
		t.Fatal(states, err)
	}
	// Failed queue validation must roll back the cursor too.
	in.Change.Sequence = 2
	in.Operation.IdempotencyKey = "bad-event"
	in.Operation.RequestFingerprint = "bad"
	if _, err = s.AcceptXNodeDDNSChange(ctx, in, now); err == nil {
		t.Fatal("invalid operation committed")
	}
	states, err = s.XNodeDDNSChanges(ctx)
	if err != nil || states[0].Change.Sequence != 1 {
		t.Fatal(states, err)
	}
	// Maintenance cannot fail or delete a pending desired mutation receipt.
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	counts := map[string]int64{}
	if err = pruneProviderOperationsTx(ctx, tx, now.Add(time.Hour), now.Add(48*time.Hour), counts); err != nil {
		t.Fatal(err)
	}
	var status string
	if err = tx.QueryRowContext(ctx, `SELECT status FROM provider_operations WHERE id=?`, states[0].OperationID).Scan(&status); err != nil || status != "queued" {
		t.Fatal(fmt.Sprint(counts), status, err)
	}
}
