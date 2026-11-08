package database

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestPurgeScrubsOnlyOwnerOperationalDataAndArchivesPaidGrant(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u, other := createTestUser(t, s, 497861), createTestUser(t, s, 497862)
	now := time.Now().UTC()
	for _, owner := range []string{u.ID, other.ID} {
		identity, err := s.SaveXNodeIdentity(ctx, xnode.Identity{UserID: owner, CredentialsCipher: "private-credentials"})
		if err != nil {
			t.Fatal(err)
		}
		op, _, err := s.CreateProviderOperation(ctx, cleanupOperationInput(owner), now)
		if err != nil {
			t.Fatal(err)
		}
		completeTestProviderOperation(t, s, op.Receipt.ID, now)
		if _, err = s.DB().ExecContext(ctx, `INSERT INTO xnode_routing_plans
 (proxy_user_id,cipher,revision,phase,operation_id,updated_at) VALUES($1,'private-routing',1,'disabled',$2,$3)`, identity.ProxyUserID, op.Receipt.ID, stamp(now)); err != nil {
			t.Fatal(err)
		}
		if _, err = s.DB().ExecContext(ctx, `INSERT INTO user_preferences(user_id) VALUES($1) ON CONFLICT DO NOTHING`, owner); err != nil {
			t.Fatal(err)
		}
		if _, err = s.DB().ExecContext(ctx, `INSERT INTO hourly_account_preferences(owner_id,account_generation,updated_at) VALUES($1,0,$2)`, owner, now); err != nil {
			t.Fatal(err)
		}
		if _, err = s.DB().ExecContext(ctx, `INSERT INTO activity_extension_credits(id,user_id,days,source_type,source_id,created_at)
 VALUES($1,$1,2,'fixture',$1,$2)`, owner, stamp(now)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.AdjustBalance(ctx, u.ID, 500, "scrub-seed", "fixture", now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, `INSERT INTO xnode_firewall_exemptions
 (id,user_id,idempotency_key,ip,price_minor,hours,until_at,created_at,issued_version,released_version,ledger_id)
 SELECT 'legacy-paid',$1,'original-checkout','192.0.2.46',100,24,$2,$3,1,2,id
 FROM ledger_entries WHERE user_id=$1 AND reference_id='scrub-seed'`, u.ID, stamp(now.Add(-time.Hour)), stamp(now.Add(-25*time.Hour))); err != nil {
		t.Fatal(err)
	}
	admitTestAccountCleanup(t, s, u, now)
	if err := s.CompleteAccountPurge(ctx, u.ID, 1, now); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`SELECT count(*) FROM user_preferences WHERE user_id=$1`,
		`SELECT count(*) FROM hourly_account_preferences WHERE owner_id=$1`,
		`SELECT count(*) FROM xnode_routing_plans WHERE proxy_user_id=(SELECT proxy_user_id FROM xnode_identities WHERE user_id=$1)`,
		`SELECT count(*) FROM xnode_firewall_exemptions WHERE user_id=$1`,
	} {
		var count int
		if err := s.DB().QueryRowContext(ctx, query, u.ID).Scan(&count); err != nil || count != 0 {
			t.Fatal("owner operational data retained", count, err)
		}
	}
	for _, owner := range []struct {
		id     string
		purged bool
	}{{u.ID, true}, {other.ID, false}} {
		identity, err := s.XNodeIdentityByUser(ctx, owner.id)
		if err != nil || (identity.CredentialsCipher == "") != owner.purged {
			t.Fatal("credential isolation failed", identity.ProxyUserID, err)
		}
		var consumed bool
		if err = s.DB().QueryRowContext(ctx, `SELECT consumed_at IS NOT NULL FROM activity_extension_credits WHERE user_id=$1`, owner.id).Scan(&consumed); err != nil || consumed != owner.purged {
			t.Fatal("extension benefit resurrected or foreign benefit removed", consumed, err)
		}
	}
	var foreign int
	if err := s.DB().QueryRowContext(ctx, `SELECT count(*) FROM xnode_routing_plans WHERE proxy_user_id=(SELECT proxy_user_id FROM xnode_identities WHERE user_id=$1)`, other.ID).Scan(&foreign); err != nil || foreign != 1 {
		t.Fatal("foreign routing removed", foreign, err)
	}
	var detail string
	if err := s.DB().QueryRowContext(ctx, `SELECT detail FROM audit_events WHERE id='account-purge-firewall:legacy-paid'`).Scan(&detail); err != nil {
		t.Fatal(err)
	}
	var receipt map[string]any
	if err := json.Unmarshal([]byte(detail), &receipt); err != nil || receipt["priceMinor"] != "100" || receipt["hours"] != "24" || receipt["idempotencyKey"] != "original-checkout" || receipt["ledgerId"] == nil || receipt["ip"] != nil {
		t.Fatal("paid grant evidence not safely retained", receipt, err)
	}
	var original int64
	if err := s.DB().QueryRowContext(ctx, `SELECT delta_txb_minor FROM ledger_entries WHERE user_id=$1 AND reference_id='scrub-seed'`, u.ID).Scan(&original); err != nil || original != 500 {
		t.Fatal("immutable ledger altered", original, err)
	}
}

func TestLiveCheckoutPreventsAccountFinancialPurge(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497863)
	now := time.Now().UTC()
	order := createTestPaymentOrder(t, s, u.ID, "ezpay", 500, now)
	admitTestAccountCleanup(t, s, u, now)
	if reason, err := s.AccountPurgeDependency(ctx, u.ID, 1, now); err != nil || reason != "account_purge_payment_outcome_pending" {
		t.Fatal(reason, err)
	}
	if err := s.CompleteAccountPurge(ctx, u.ID, 1, now); err == nil {
		t.Fatal("live checkout lost its settlement boundary")
	}
	var status string
	if err := s.DB().QueryRowContext(ctx, `SELECT status FROM payment_orders WHERE id=$1`, order.ID).Scan(&status); err != nil || status != "pending" {
		t.Fatal("live checkout was discarded", status, err)
	}
}
