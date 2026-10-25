package xnode_test

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/preferences"
)

func TestPAYGSquadOptOutRemovesInventoryAndCompilation(t *testing.T) {
	f := controlFixture(t, false)
	ctx := context.Background()
	provisionPAYGFixture(t, f)
	if _, err := f.store.UpdateUserPreferences(ctx, f.user.ID, preferences.Patch{PAYGSquads: map[string]bool{f.group.ID: false}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	proxies, err := f.service.MemberProxies(ctx, f.user.ID)
	if err != nil || len(proxies) != 0 {
		t.Fatal(proxies, err)
	}
	compiled, err := f.service.Compile(ctx, f.node.ID)
	if err != nil || len(compiled.Users) != 0 {
		t.Fatal(compiled.Users, err)
	}
	view, err := f.service.OwnedPAYG(ctx, f.user.ID)
	if err != nil || len(view.Squads) != 1 || view.Squads[0].Status != "disabled" {
		t.Fatal(view, err)
	}
	if _, err := f.store.UpdateUserPreferences(ctx, f.user.ID, preferences.Patch{PAYGSquads: map[string]bool{f.group.ID: true}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	proxies, err = f.service.MemberProxies(ctx, f.user.ID)
	if err != nil || len(proxies) != 1 {
		t.Fatal(proxies, err)
	}
}

func TestPAYGMasterOptOutPreservesPaidComboAccess(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	provisionPAYGFixture(t, f)
	off := false
	if _, err := f.store.UpdateUserPreferences(ctx, f.user.ID, preferences.Patch{PAYGEnabled: &off}, time.Now()); err != nil {
		t.Fatal(err)
	}
	compiled, err := f.service.Compile(ctx, f.node.ID)
	if err != nil || len(compiled.Users) != 1 || compiled.Users[0].ID != int(f.identity.ProxyUserID) {
		t.Fatal(compiled.Users, err)
	}
}
