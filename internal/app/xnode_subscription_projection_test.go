package app

import (
	"slices"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

func TestSplitXHTTPLinksDoNotChangeSquadMonitoringMembership(t *testing.T) {
	now := time.Now().UTC()
	up := subscription.Proxy{ID: "up", NodeID: "node", InboundTag: "paid", GroupIDs: []string{"enabled", "disabled"}, Name: "Upload", Protocol: "vless", Address: "up.example", Port: 443,
		Account: map[string]any{"id": "11111111-1111-4111-8111-111111111111"}, Stream: map[string]any{"network": "xhttp", "xhttpSettings": map[string]any{"path": "/shared"}}}
	down := up
	down.ID, down.Name, down.Address = "down", "Download", "down.example"
	split, err := subscription.SplitProxy("custom", "packet-up", up, down)
	if err != nil {
		t.Fatal(err)
	}
	monitored := map[string]bool{up.ID: true, down.ID: true}
	for _, links := range []bool{false, true} {
		proxies := []subscription.Proxy{up, down}
		if links {
			proxies = append(proxies, split)
		}
		view := projectMemberSubscription(proxies, monitored, []string{"enabled"}, map[string]string{"enabled": "Paid"}, links, now)
		if len(view.Squads) != 1 || !slices.Equal(view.Squads[0].HostUUIDs, []string{"up", "down"}) {
			t.Fatal("derived profile or disabled group entered squad status", view.Squads)
		}
		if links && (len(view.Hosts) != 3 || view.Hosts[2].UUID != split.ID || view.Hosts[2].Link == nil) {
			t.Fatal("split import link disappeared", view.Hosts)
		}
		// Unmeasured split state must not turn measured published hosts unknown.
		byHost := map[string]connectivity.UptimeTimeline{}
		for _, host := range view.Hosts {
			timeline := connectivity.UnknownTimeline(now)
			if monitored[host.UUID] {
				timeline.State = "operational"
			}
			byHost[host.UUID] = timeline
		}
		timelines := []connectivity.UptimeTimeline{}
		for _, id := range view.Squads[0].HostUUIDs {
			timelines = append(timelines, byHost[id])
		}
		if got := connectivity.AggregateTimelines(timelines, now).State; got != "operational" {
			t.Fatal("split profile contaminated squad status", got)
		}
	}
}
