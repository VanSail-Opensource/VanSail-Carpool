package xnode

import (
	"slices"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

func TestSplitNamesRemainUniqueForReversePairsAndOrdinaryHosts(t *testing.T) {
	fixture := func(id, name string) subscription.Proxy {
		return subscription.Proxy{ID: id, Name: name, NodeID: "node", InboundTag: "xhttp", Protocol: "vless", Address: id + ".example", Port: 443,
			Account: map[string]any{"id": "c1d50c69-2f17-41ec-a23c-b30eb375639f", "encryption": "none"},
			Stream:  map[string]any{"network": "xhttp", "xhttpSettings": map[string]any{"path": "/shared"}}}
	}
	profiles := []SplitProfile{{ID: "forward", UploadEndpointID: "v4", DownloadEndpointID: "v6", Mode: "auto"}, {ID: "reverse", UploadEndpointID: "v6", DownloadEndpointID: "v4", Mode: "auto"}}
	for _, collision := range []bool{false, true} {
		ordinary := []subscription.Proxy{fixture("v4", "Shared-V4"), fixture("v6", "Shared-V6")}
		if collision {
			ordinary = append(ordinary, fixture("other", "Shared-V4V6"), fixture("suffix", "Shared-V4V6 (2)"))
		}
		result := appendSplitProxies(profiles, ordinary)
		if len(result) != len(ordinary)+2 {
			t.Fatal("profiles lost", result)
		}
		for i, p := range ordinary {
			if result[i].Name != p.Name || result[i].ID != p.ID {
				t.Fatal("ordinary host changed")
			}
		}
		want := "Shared-V4V6"
		if collision {
			want += " (3)"
		}
		if result[len(ordinary)].Name != want || result[len(ordinary)].ID != "split:forward" || result[len(ordinary)+1].ID != "split:reverse" {
			t.Fatal("combined name or identity changed", result)
		}
		reversed := slices.Clone(profiles)
		slices.Reverse(reversed)
		again := appendSplitProxies(reversed, ordinary)
		for index, proxy := range result {
			if again[index].ID != proxy.ID || again[index].Name != proxy.Name {
				t.Fatal("profile order changed export names")
			}
		}
		for _, format := range []string{"xray", "clash", "stash"} {
			if _, err := subscription.RenderTemplate(subscription.DefaultTemplate(format), result); err != nil {
				t.Fatal(format, err)
			}
		}
	}
}
