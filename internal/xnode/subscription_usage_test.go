package xnode_test

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestSubscriptionInfoPreservesComboAndPrepaidPAYGSnapshots(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		combo, payg, expiredCombo bool
		expiredCredit             bool
		used, limit               string
	}{
		{name: "combo", combo: true, used: "321", limit: "2048"},
		{name: "combo-and-payg", combo: true, payg: true, used: "321", limit: "2048"},
		{name: "payg", payg: true, used: "134", limit: "734"},
		{name: "payg-after-combo", combo: true, payg: true, expiredCombo: true, used: "134", limit: "734"},
		{name: "payg-expired-credit", payg: true, expiredCredit: true, used: "134", limit: "134"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := controlFixture(t, tc.combo)
			ctx := context.Background()
			if _, err := f.service.SaveEndpoint(ctx, xnode.Endpoint{NodeID: f.node.ID, InboundTag: "paid", Name: "Paid host", Address: "proxy.example", Port: 443, Enabled: true}); err != nil {
				t.Fatal(err)
			}
			identity, err := f.store.XNodeIdentityByUser(ctx, f.user.ID)
			if err != nil {
				t.Fatal(err)
			}
			if tc.combo {
				if _, err := f.store.DB().ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=321,limit_bytes=2048 WHERE id=?`, identity.CurrentPeriodID); err != nil {
					t.Fatal(err)
				}
			}
			if tc.expiredCombo {
				if _, err := f.store.DB().ExecContext(ctx, `UPDATE purchases SET status='expired',valid_until=? WHERE user_id=?`, time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano), f.user.ID); err != nil {
					t.Fatal(err)
				}
			}
			if tc.payg {
				provisionPAYGFixture(t, f)
				group, err := f.service.SaveGroup(ctx, xnode.Group{Name: "Extra PAYG", Enabled: true, Bindings: []xnode.GroupBinding{{NodeID: f.node.ID, InboundTag: "paid"}}})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := f.store.SaveSquadProduct(ctx, database.SquadProductInput{GroupID: group.ID, UpstreamPresent: true, BillingMode: "payg", PAYGPriceTXBMinor: 100}); err != nil {
					t.Fatal(err)
				}
				payload, err := json.Marshal(map[string]string{"userId": f.user.ID})
				if err != nil {
					t.Fatal(err)
				}
				if err := f.service.ReconcilePAYG(ctx, model.OutboxJob{Kind: xnode.PAYGJob, Payload: string(payload)}); err != nil {
					t.Fatal(err)
				}
				accounts, err := f.store.XNodePAYGAccounts(ctx, f.user.ID, "")
				if err != nil || len(accounts) != 2 {
					t.Fatalf("PAYG fixture: %d accounts: %v", len(accounts), err)
				}
				for index, account := range accounts {
					if _, err := f.store.DB().ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=? WHERE id=?`, 64+index*6, account.PeriodID); err != nil {
						t.Fatal(err)
					}
					started := time.Now().UTC()
					if index > 0 || tc.expiredCredit {
						started = started.Add(-xnode.PAYGWindow)
					}
					if _, err := f.store.DB().ExecContext(ctx, `UPDATE xnode_payg_accounts SET purchased_bytes=?,credit_bytes=?,credit_started_at=? WHERE proxy_user_id=?`, 3*xnode.PAYGGB, 600+index*200, started.Format(time.RFC3339Nano), account.ProxyUserID); err != nil {
						t.Fatal(err)
					}
				}
			}
			metadata, err := f.service.OwnedSubscription(ctx, f.user.ID)
			if err != nil || !metadata.Active || metadata.UsedBytes != tc.used || metadata.LimitBytes != tc.limit {
				t.Fatalf("metadata = %+v, error = %v", metadata, err)
			}
			if (metadata.ExpiresAt != nil) != (tc.combo && !tc.expiredCombo) {
				t.Fatalf("wrong expiry: %v", metadata.ExpiresAt)
			}
			link, err := url.Parse(metadata.DownloadURL)
			if err != nil {
				t.Fatal(err)
			}
			token := link.Path[strings.LastIndex(link.Path, "/")+1:]
			raw, downloaded, err := f.service.BearerSubscriptionForAgent(ctx, token, "clash", "FlClash")
			if err != nil || len(raw) == 0 || downloaded.UsedBytes != tc.used || downloaded.LimitBytes != tc.limit {
				t.Fatalf("download lost subscription info: %+v: %v", downloaded, err)
			}
		})
	}
}
