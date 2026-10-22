package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestWhitelistModeRejectsNewUnbanButReplaysExistingPurchase(t *testing.T) {
	store, identity, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if _, err := store.AdjustBalance(ctx, identity.UserID, 1000, "seed", "", now); err != nil {
		t.Fatal(err)
	}
	input := xnode.FirewallPurchase{IP: "8.8.8.8", Key: "whitelist-unban-key", Price: "1.25", Hours: 24}
	if _, err := store.PurchaseFirewallExemption(ctx, identity.UserID, input, 125, now, false); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("whitelist mode accepted new paid unban", err)
	}
	first, err := store.PurchaseFirewallExemption(ctx, identity.UserID, input, 125, now, true)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := store.PurchaseFirewallExemption(ctx, identity.UserID, input, 125, now, false)
	if err != nil || replay.ID != first.ID {
		t.Fatal("charged retry rejected after whitelist activation", err)
	}
	var debits int
	if err = store.DB().QueryRow(`SELECT COUNT(*) FROM ledger_entries WHERE user_id=? AND kind='firewall_exemption'`, identity.UserID).Scan(&debits); err != nil || debits != 1 {
		t.Fatal("duplicate or missing unban debit", debits, err)
	}
}
