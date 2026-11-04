package app

import (
	"context"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

func TestUnifiedSubscriptionWithoutNativeIdentityIsEmpty(t *testing.T) {
	fixture := newNativePurchaseFixture(t)
	ctx := context.Background()
	user, _, err := fixture.store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 89021, FirstName: "New member"}, false)
	if err != nil {
		t.Fatal(err)
	}
	view, err := fixture.provider.LoadUnifiedSubscription(ctx, user, true)
	if err != nil || view.SubscriptionURL != nil || view.Hosts == nil || len(view.Hosts) != 0 {
		t.Fatal("unprovisioned member did not receive an empty subscription", view, err)
	}
}

func TestSplitXHTTPLinksRetainOwnerSquadFiltering(t *testing.T) {
	up := subscription.Proxy{ID: "up", NodeID: "node", InboundTag: "paid", GroupIDs: []string{"enabled", "disabled"}, Name: "Upload", Protocol: "vless", Address: "up.example", Port: 443,
		Account: map[string]any{"id": "11111111-1111-4111-8111-111111111111"}, Stream: map[string]any{"network": "xhttp", "xhttpSettings": map[string]any{"path": "/shared"}}}
	down := up
	down.ID, down.Name, down.Address = "down", "Download", "down.example"
	split, err := subscription.SplitProxy("custom", "packet-up", up, down)
	if err != nil {
		t.Fatal(err)
	}
	for _, links := range []bool{false, true} {
		proxies := []subscription.Proxy{up, down}
		if links {
			proxies = append(proxies, split)
		}
		view := projectMemberSubscription(proxies, []string{"enabled"}, links)
		if len(view.Hosts) < 2 || len(view.Hosts[0].SquadUUIDs) != 1 || view.Hosts[0].SquadUUIDs[0] != "enabled" {
			t.Fatal("disabled group entered host projection", view.Hosts)
		}
		if links && (len(view.Hosts) != 3 || view.Hosts[2].UUID != split.ID || view.Hosts[2].Link == nil) {
			t.Fatal("split import link disappeared", view.Hosts)
		}

	}
}
