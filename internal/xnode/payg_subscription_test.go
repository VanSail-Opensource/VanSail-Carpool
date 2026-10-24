package xnode_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"

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
