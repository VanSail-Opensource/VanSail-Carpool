package xnode_test

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestBulkCompileMembersPreserveEntitlementsAndFreshUsage(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	for _, used := range []int64{123, 456} {
		identity, err := f.store.XNodeIdentityByUser(ctx, f.user.ID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.store.DB().ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=? WHERE id=?`, used, identity.CurrentPeriodID); err != nil {
			t.Fatal(err)
		}
		inputs, err := f.store.XNodeCompileMembers(ctx, []string{f.user.ID})
		if err != nil {
			t.Fatal(err)
		}
		user, err := f.store.UserByID(ctx, f.user.ID)
		if err != nil {
			t.Fatal(err)
		}
		purchases, err := f.store.ListPurchases(ctx, f.user.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(inputs.Users[f.user.ID], user) || !reflect.DeepEqual(inputs.Purchases[f.user.ID], purchases) {
			t.Fatal("bulk projection diverged from authoritative readers")
		}
		if inputs.Periods[identity.CurrentPeriodID].UsedBytes != used {
			t.Fatal("usage cached across compilations")
		}
		owners, err := f.store.XNodeObservedPrincipalOwners(ctx, []int{int(identity.ProxyUserID)})
		if err != nil || owners[int(identity.ProxyUserID)] != f.user.ID {
			t.Fatalf("owner projection: %v %v", owners, err)
		}
		purchase, err := f.store.DesiredEntitlement(ctx, f.user.ID, time.Now())
		if err != nil || purchase == nil {
			t.Fatal(err)
		}
	}
}
