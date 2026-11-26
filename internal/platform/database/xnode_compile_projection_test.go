package database

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestCompileIdentityProjectionPreservesNodeScopeAndUniqueOwners(t *testing.T) {
	s, member, _, payg := paygBillingFixture(t)
	ctx := context.Background()
	owner, err := s.UserByID(ctx, member.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO member_identity_aliases(old_user_id,user_id,old_proxy_id,proxy_id) VALUES('previous-member',$1,42,$2)`, owner.ID, member.ProxyUserID); err != nil {
		t.Fatal(err)
	}
	service, err := s.SaveXNodeIdentity(ctx, xnode.Identity{ServiceName: "ordinary-service", CredentialsCipher: "opaque"})
	if err != nil {
		t.Fatal(err)
	}
	node, err := s.XNodeNode(ctx, payg.NodeID)
	if err != nil {
		t.Fatal(err)
	}
	other := node
	other.ID, other.Name = "another-node", "Another node"
	if err := s.SaveXNodeNode(ctx, other); err != nil {
		t.Fatal(err)
	}
	group := xnode.Group{ID: "hourly-test", Name: "Hourly test", Enabled: true,
		Bindings: []xnode.GroupBinding{{NodeID: node.ID, InboundTag: "one"}, {NodeID: node.ID, InboundTag: "two"}}}
	if err := s.SaveXNodeGroup(ctx, group); err != nil {
		t.Fatal(err)
	}
	hourly := hourlyBreakdownEnrollment(t, s, owner, "hourly-test-principal", group.ID, time.Now().UTC())
	for _, tc := range []struct {
		node string
		ids  []int64
	}{
		{node.ID, []int64{member.ProxyUserID, payg.ProxyUserID, service.ProxyUserID, hourly.ProxyUserID}},
		{other.ID, []int64{member.ProxyUserID, service.ProxyUserID}},
	} {
		t.Run(tc.node, func(t *testing.T) {
			got, err := s.XNodeIdentitiesForNode(ctx, tc.node)
			if err != nil {
				t.Fatal(err)
			}
			want := make(map[int64]xnode.Identity, len(tc.ids))
			for _, id := range tc.ids {
				identity, err := s.XNodeIdentity(ctx, id)
				if err != nil {
					t.Fatal(err)
				}
				want[id] = identity
			}
			if len(got) != len(want) {
				t.Fatalf("node projection length = %d, want %d", len(got), len(want))
			}
			var previous int64
			for _, identity := range got {
				if identity.ProxyUserID <= previous || !reflect.DeepEqual(identity, want[identity.ProxyUserID]) {
					t.Fatalf("unexpected projection for principal %d", identity.ProxyUserID)
				}
				previous = identity.ProxyUserID
			}
		})
	}
}

func TestCompileMemberProjectionExcludesRetainedPeriodHistory(t *testing.T) {
	ctx := context.Background()
	s, identity, current := xnodeTrafficFixture(t)
	ended := current.StartedAt
	history := xnode.Period{ID: "retained-previous", ProxyUserID: identity.ProxyUserID, StartedAt: ended.Add(-time.Hour),
		EndedAt: &ended, Strategy: "NO_RESET", Status: "closed"}
	if err := s.SaveXNodePeriod(ctx, history); err != nil {
		t.Fatal(err)
	}
	result, err := s.XNodeCompileMembers(ctx, []string{identity.UserID})
	if err != nil || len(result.Periods) != 1 || result.Periods[current.ID].ID != current.ID {
		t.Fatalf("unexpected compile period projection size %d, error=%v", len(result.Periods), err)
	}
	retained, err := s.XNodePeriod(ctx, history.ID)
	if err != nil || retained.Status != "closed" {
		t.Fatalf("excluded historical period did not remain available: %v", err)
	}
}
