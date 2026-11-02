package xnode_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestPAYGSharedEndpointRendersAllFormatsWithDistinctCredentials(t *testing.T) {
	for _, paid := range []bool{false, true} {
		t.Run(fmt.Sprintf("combo=%v", paid), func(t *testing.T) {
			f := controlFixture(t, paid)
			ctx := context.Background()
			provisionPAYGFixture(t, f)
			group, err := f.service.SaveGroup(ctx, xnode.Group{Name: "Second PAYG group", Enabled: true, Bindings: []xnode.GroupBinding{{NodeID: f.node.ID, InboundTag: "paid"}}})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.store.SaveSquadProduct(ctx, database.SquadProductInput{GroupID: group.ID, UpstreamPresent: true, BillingMode: "payg", PAYGPriceTXBMinor: 200}); err != nil {
				t.Fatal(err)
			}
			payload, err := json.Marshal(map[string]string{"userId": f.user.ID})
			if err != nil {
				t.Fatal(err)
			}
			if err = f.service.ReconcilePAYG(ctx, model.OutboxJob{Kind: xnode.PAYGJob, Payload: string(payload)}); err != nil {
				t.Fatal(err)
			}
			proxies, err := f.service.MemberProxies(ctx, f.user.ID)
			if err != nil || len(proxies) != 2 {
				t.Fatalf("shared endpoint inventory count=%d: %v", len(proxies), err)
			}
			if proxies[0].Name != proxies[1].Name || proxies[0].ID == proxies[1].ID || proxies[0].Account["id"] == proxies[1].Account["id"] {
				t.Fatal("fixture must retain separate credentials sharing one endpoint label")
			}
			balance, err := f.store.Balance(ctx, f.user.ID)
			if err != nil || balance.MinorInt64() != 0 {
				t.Fatalf("fixture must begin at zero balance: %+v, %v", balance, err)
			}
			accountsBefore, err := f.store.XNodePAYGAccounts(ctx, f.user.ID, "")
			if err != nil || len(accountsBefore) == 0 {
				t.Fatal("missing PAYG accounts", err)
			}
			var creditPrincipal int64
			for _, proxy := range proxies {
				if proxy.PrincipalID != 0 {
					creditPrincipal = proxy.PrincipalID
					break
				}
			}
			if creditPrincipal == 0 {
				t.Fatal("fixture must have an exported PAYG principal")
			}
			if _, err = f.store.DB().ExecContext(ctx, `UPDATE xnode_payg_accounts SET purchased_bytes=?,credit_bytes=?,credit_started_at=? WHERE proxy_user_id=?`, xnode.PAYGGB, 123, time.Now().UTC().Format(time.RFC3339Nano), creditPrincipal); err != nil {
				t.Fatal(err)
			}
			accountsBefore, err = f.store.XNodePAYGAccounts(ctx, f.user.ID, "")
			if err != nil {
				t.Fatal(err)
			}
			var hasCredit bool
			for _, account := range accountsBefore {
				hasCredit = hasCredit || account.ProxyUserID == creditPrincipal && account.HasCredit(time.Now())
			}
			if !hasCredit {
				t.Fatal("fixture must contain valid prepaid credit", err)
			}
			for _, step := range []struct {
				name        string
				delta       int64
				balance     int64
				paygVisible bool
			}{{"negative", -1, -1, false}, {"zero", 1, 0, false}, {"positive", 1, 1, true}} {
				if _, err = f.store.AdjustBalance(ctx, f.user.ID, step.delta, "subscription-"+step.name, "test", time.Now()); err != nil {
					t.Fatal(err)
				}
				balance, err = f.store.Balance(ctx, f.user.ID)
				if err != nil || balance.MinorInt64() != step.balance {
					t.Fatalf("%s: unexpected balance: %+v, %v", step.name, balance, err)
				}
				exported, err := f.service.ExportProxiesForUser(ctx, f.user.ID, proxies)
				if err != nil {
					t.Fatal(err)
				}
				want := 0
				if paid {
					want++
				}
				if step.paygVisible {
					want = len(proxies)
				}
				if len(exported) != want {
					t.Fatalf("%s: got %d exports, want %d", step.name, len(exported), want)
				}
				for _, proxy := range exported {
					if !step.paygVisible && proxy.PrincipalID != 0 {
						t.Fatalf("%s: PAYG export remained with nonpositive balance", step.name)
					}
				}
				if paid {
					metadata, err := f.service.OwnedSubscription(ctx, f.user.ID)
					if err != nil {
						t.Fatal(err)
					}
					link, err := url.Parse(metadata.DownloadURL)
					if err != nil {
						t.Fatal(err)
					}
					token := link.Path[strings.LastIndex(link.Path, "/")+1:]
					raw, _, err := f.service.BearerSubscription(ctx, token, "native")
					if err != nil {
						t.Fatal(err)
					}
					for _, proxy := range proxies {
						visible := proxy.PrincipalID == 0 || step.paygVisible
						if strings.Contains(string(raw), proxy.Account["id"].(string)) != visible {
							t.Fatalf("%s: subscription had incorrect credential visibility", step.name)
						}
					}
				}
			}
			accountsAfter, err := f.store.XNodePAYGAccounts(ctx, f.user.ID, "")
			if err != nil || len(accountsAfter) != len(accountsBefore) {
				t.Fatal("balance changes altered PAYG accounts", accountsAfter, err)
			}
			for i := range accountsBefore {
				if accountsAfter[i].ProxyUserID != accountsBefore[i].ProxyUserID || accountsAfter[i].PeriodID != accountsBefore[i].PeriodID || accountsAfter[i].CreditBytes != accountsBefore[i].CreditBytes || accountsAfter[i].PurchasedBytes != accountsBefore[i].PurchasedBytes || !accountsAfter[i].CreditStartedAt.Equal(accountsBefore[i].CreditStartedAt) {
					t.Fatal("subscription export altered PAYG identity or credit")
				}
			}
			metadata, err := f.service.OwnedSubscription(ctx, f.user.ID)
			if err != nil {
				t.Fatal(err)
			}
			link, err := url.Parse(metadata.DownloadURL)
			if err != nil {
				t.Fatal(err)
			}
			token := link.Path[strings.LastIndex(link.Path, "/")+1:]
			for _, format := range []string{"native", "base64", "xray", "sing-box", "clash", "stash"} {
				if !metadata.Formats[format] || metadata.FormatDiagnostics[format].Code != "" {
					t.Fatal("PAYG disabled subscription format", format, metadata.FormatDiagnostics[format])
				}
				raw, _, err := f.service.BearerSubscription(ctx, token, format)
				if err != nil || len(raw) == 0 {
					t.Fatalf("%s download failed: %v", format, err)
				}
				if format == "base64" {
					raw, err = base64.StdEncoding.DecodeString(string(raw))
					if err != nil {
						t.Fatal(err)
					}
				}
				for _, proxy := range proxies {
					if !strings.Contains(string(raw), proxy.Account["id"].(string)) {
						t.Fatal("download dropped a billing identity", format)
					}
				}
			}
		})
	}
}
