package xnode_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestPAYGAdminStatesPreserveIdentityAndOnlyUnavailableRevokes(t *testing.T) {
	f := controlFixture(t, false)
	ctx := context.Background()
	provisionPAYGFixture(t, f)
	accounts, err := f.store.XNodePAYGAccounts(ctx, f.user.ID, "")
	if err != nil || len(accounts) == 0 {
		t.Fatal(accounts, err)
	}
	original, err := f.service.Compile(ctx, f.node.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"hidden", "unavailable", "normal"} {
		if _, err := f.store.SaveSquadProduct(ctx, database.SquadProductInput{GroupID: f.group.ID,
			UpstreamPresent: true, BillingMode: "payg", PAYGPriceTXBMinor: 100, PAYGState: state}); err != nil {
			t.Fatal(err)
		}
		proxies, err := f.service.MemberProxies(ctx, f.user.ID)
		if err != nil || (len(proxies) > 0) != (state != "unavailable") {
			t.Fatal("incorrect state access", state, proxies, err)
		}
		exported, err := f.service.ExportProxiesForUser(ctx, f.user.ID, proxies)
		if err != nil || (len(exported) > 0) != (state == "normal") {
			t.Fatal("incorrect subscription visibility", state, exported, err)
		}
		compiled, err := f.service.Compile(ctx, f.node.ID)
		if err != nil || (len(compiled.Users) > 0) != (state != "unavailable") {
			t.Fatal("incorrect compiled access", state, compiled.Users, err)
		}
		if state == "hidden" && !reflect.DeepEqual(compiled.Users, original.Users) {
			t.Fatal("hiding changed live authorization or credentials")
		}
		after, err := f.store.XNodePAYGAccounts(ctx, f.user.ID, "")
		if err != nil || len(after) != len(accounts) || after[0].ProxyUserID != accounts[0].ProxyUserID || after[0].PeriodID != accounts[0].PeriodID {
			t.Fatal("state changed accounting identity", after, err)
		}
	}
}
