package database

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/preferences"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestPAYGPreferencesPreserveSquadChoicesAcrossMasterSwitch(t *testing.T) {
	s, owner, _, account := paygBillingFixture(t)
	ctx, now := context.Background(), time.Now()
	if err := s.SaveXNodeGroup(ctx, xnode.Group{ID: account.GroupID, Name: "PAYG", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	off, on := false, true
	p, err := s.UpdateUserPreferences(ctx, owner.UserID, preferences.Patch{PAYGSquads: map[string]bool{account.GroupID: false}}, now)
	if err != nil || !p.PAYGEnabled || p.PAYGSquads[account.GroupID] {
		t.Fatal(p, err)
	}
	for _, enabled := range []*bool{&off, &on} {
		p, err = s.UpdateUserPreferences(ctx, owner.UserID, preferences.Patch{PAYGEnabled: enabled}, now)
		if err != nil || p.PAYGSquads[account.GroupID] || len(p.PAYGSquads) != 1 {
			t.Fatal(p, err)
		}
	}
	choice, err := s.XNodePAYGPreferences(ctx, owner.UserID)
	if err != nil || choice.Allows(account.GroupID) || !choice.Allows("new-squad") {
		t.Fatal(choice, err)
	}
	p, err = s.UpdateUserPreferences(ctx, owner.UserID, preferences.Patch{PAYGSquads: map[string]bool{account.GroupID: true}}, now)
	if err != nil || len(p.PAYGSquads) != 0 {
		t.Fatal(p, err)
	}
}

func TestPAYGOptOutBlocksConnectionPurchaseButSettlesDeliveredTraffic(t *testing.T) {
	s, owner, _, account := paygBillingFixture(t)
	ctx, now := context.Background(), time.Now().UTC()
	off := false
	if _, err := s.UpdateUserPreferences(ctx, owner.UserID, preferences.Patch{PAYGEnabled: &off}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, `INSERT INTO xnode_payg_windows(proxy_user_id,through,traffic_bytes,unbilled_bytes) VALUES(?,?,?,?)`, account.ProxyUserID, paygWindowStamp(now), xnode.PAYGStartBytes, xnode.PAYGStartBytes); err != nil {
		t.Fatal(err)
	}
	if err := s.ObserveXNodePAYGConnections(ctx, account.NodeID, []int64{account.ProxyUserID}, now); err != nil {
		t.Fatal(err)
	}
	paused, err := s.XNodePAYGAccount(ctx, account.ProxyUserID)
	if err != nil || !paused.Disabled || paused.PurchasedBytes != 0 {
		t.Fatal(paused, err)
	}
	paygReport(t, s, account, 1, 1)
	settled, err := s.XNodePAYGAccount(ctx, account.ProxyUserID)
	if err != nil || settled.UsedBytes != 1 || settled.PurchasedBytes != xnode.PAYGGB || !settled.Disabled {
		t.Fatal(settled, err)
	}
}

func TestPAYGConcurrentPreferencesMergeWithoutLostUpdates(t *testing.T) {
	s, owner, _, _ := paygBillingFixture(t)
	ctx, now := context.Background(), time.Now()
	off := false
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, patch := range []preferences.Patch{{PAYGEnabled: &off}, {IncludeNodePrices: &off}} {
		wg.Add(1)
		go func(p preferences.Patch) {
			defer wg.Done()
			_, err := s.UpdateUserPreferences(ctx, owner.UserID, p, now)
			errs <- err
		}(patch)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	p, err := s.UserPreferences(ctx, owner.UserID)
	if err != nil || p.PAYGEnabled || p.IncludeNodePrices {
		t.Fatal(p, err)
	}
}
