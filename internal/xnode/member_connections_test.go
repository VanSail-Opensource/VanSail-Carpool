package xnode

import (
	"context"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestMemberOpenConnectionsDeterminePresenceWithoutChangingSampledCount(t *testing.T) {
	for _, test := range []struct {
		name       string
		active     int
		open       int
		negotiated bool
		stale      bool
		status     string
		total      string
	}{
		{"idle open", 0, 5, true, false, "online", "5"},
		{"active open", 2, 5, true, false, "online", "5"},
		{"closed with residual rate", 0, 0, true, false, "offline", "0"},
		{"stale open", 0, 5, true, true, "unavailable", ""},
		{"legacy active", 2, 0, false, false, "online", ""},
		{"legacy idle", 0, 0, false, false, "unavailable", ""},
		{"unnegotiated total", 0, 5, false, false, "unavailable", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			master, now := profileMetricMaster(t, "native")
			master.peers["node"].openConnections = test.negotiated
			snapshot := master.live["node"]
			snapshot.report.Users.Connections = map[int]int{1: test.active}
			snapshot.report.Users.TotalConnections = map[int]int{1: test.open}
			if test.name != "closed with residual rate" {
				snapshot.report.Users.Rates = nil
			}
			if test.stale {
				snapshot.receivedAt = now.Add(-time.Minute)
			}
			master.live["node"] = snapshot
			result, err := master.UserMetrics(context.Background(), "member")
			if err != nil || result.ActivityStatus != test.status {
				t.Fatal("incorrect presence", result, err)
			}
			if test.total == "" {
				if result.TotalConnections != nil {
					t.Fatal("unknown total fabricated", result)
				}
			} else if result.TotalConnections == nil || *result.TotalConnections != test.total {
				t.Fatal("incorrect total", result)
			}
			if !test.stale && result.Connections != int64(test.active) {
				t.Fatal("traffic-active count changed", result)
			}
		})
	}
}

func TestTotalConnectionMetricsRejectInvalidCounts(t *testing.T) {
	for _, counts := range []map[int]int{{0: 1}, {1: -1}} {
		if validMetrics(wire.UserMetrics{TotalConnections: counts}) {
			t.Fatal("invalid total accepted", counts)
		}
	}
}
