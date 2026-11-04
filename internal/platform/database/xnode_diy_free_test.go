package database

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestDIYSaveDoesNotConsumeLookupAllowanceOrBalance(t *testing.T) {
	for _, quota := range []int{0, 1} {
		t.Run(fmt.Sprint(quota), func(t *testing.T) {
			s, lookup, user, purchase, now := ipLookupFixture(t, quota)
			ctx := context.Background()
			before, err := s.Balance(ctx, user)
			if err != nil {
				t.Fatal(err)
			}
			var usedBefore int
			if err = s.db.QueryRowContext(ctx, `SELECT used FROM ip_lookup_allowances WHERE purchase_id=$1`, purchase).Scan(&usedBefore); err != nil {
				t.Fatal(err)
			}
			doc := xnode.DIYDocument{UserID: user, Cipher: "encrypted-settings", Result: &xnode.DIYSaveResult{Changed: true, SettingsRevision: 1}}
			input := providerops.CreateInput{ActorUserID: user, OwnerUserID: user, Kind: xnode.DIYOperationKind, IdempotencyKey: "free-save", RequestFingerprint: strings.Repeat("f", 64), Items: []providerops.ItemInput{{Key: "settings", TargetType: "member_diy", TargetID: user}}}
			receipt, err := s.SaveXNodeDIY(ctx, doc, 0, nil, input, now)
			if err != nil || receipt.Status != "succeeded" {
				t.Fatal(receipt, err)
			}
			replay, err := s.SaveXNodeDIY(ctx, doc, 0, nil, input, now.Add(time.Hour))
			if err != nil || replay.ID != receipt.ID {
				t.Fatal("save was not replayed", replay, err)
			}
			result, found, err := s.ReplayXNodeDIYResult(ctx, user, input.IdempotencyKey, input.RequestFingerprint)
			if err != nil || !found || !result.Changed || result.SettingsRevision != 1 || result.Receipt.ID != receipt.ID {
				t.Fatal("committed save result was not replayed", result, found, err)
			}
			after, err := s.Balance(ctx, user)
			if err != nil || before.Minor != after.Minor {
				t.Fatal("DIY changed balance", before, after, err)
			}
			var usedAfter, ledger int
			if err = s.db.QueryRowContext(ctx, `SELECT used FROM ip_lookup_allowances WHERE purchase_id=$1`, purchase).Scan(&usedAfter); err != nil || usedAfter != usedBefore {
				t.Fatal("DIY consumed IP Lookup allowance", usedBefore, usedAfter, err)
			}
			if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE user_id=$1 AND kind='xnode_diy'`, user).Scan(&ledger); err != nil || ledger != 0 {
				t.Fatal("DIY created a ledger charge", ledger, err)
			}
			saved, err := s.XNodeDIY(ctx, user)
			if err != nil || saved.Revision != 1 || saved.Cipher != doc.Cipher {
				t.Fatal("DIY document not saved", saved, err)
			}
			quote, err := lookup.Quote(ctx, user, "8.8.8.8", false)
			if err != nil || quote.UseQuota != (quota > 0) {
				t.Fatal("IP Lookup policy changed", quote, err)
			}
			if _, err = lookup.Submit(ctx, user, "independent-lookup", quote); err != nil {
				t.Fatal("IP Lookup could not use its own policy", err)
			}
		})
	}
}
