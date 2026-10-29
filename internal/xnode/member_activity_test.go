package xnode

import (
	"context"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestNativeActivityDistinguishesIdleFromUnavailable(t *testing.T) {
	for _, mode := range []string{"online", "offline", "stale", "unsupported", "disabled"} {
		master, now := profileMetricMaster(t, "native")
		master.peers["node"].openConnections = true
		snapshot := master.live["node"]
		snapshot.report.Users.TotalConnections = map[int]int{1: 2}
		if mode == "offline" {
			snapshot.report.Users.Connections = nil
			snapshot.report.Users.TotalConnections = nil
			snapshot.report.Users.Rates = nil
		}
		if mode == "stale" {
			snapshot.receivedAt = now.Add(-time.Minute)
		}
		if mode == "unsupported" {
			snapshot.hello.Capabilities = nil
		}
		if mode == "disabled" {
			master.service.repo = profileMetricRepository{disabled: true, date: now.Format(time.DateOnly)}
		}
		master.live["node"] = snapshot
		result, err := master.UserMetrics(context.Background(), "member")
		if err != nil {
			t.Fatal(err)
		}
		expected := "unavailable"
		if mode == "online" || mode == "offline" {
			expected = mode
		}
		if result.ActivityStatus != expected {
			t.Fatalf("%s: %+v", mode, result)
		}
		if expected == "unavailable" && result.ActivityExpiresAt != nil {
			t.Fatal("unavailable observation has an expiry", result)
		}
		if expected != "unavailable" && (result.ActivityExpiresAt == nil || !result.ActivityExpiresAt.After(now)) {
			t.Fatal("fresh activity lacks an expiry", result)
		}
	}
}

func TestFreshTransferRateIsNativeActivityWithoutOpenConnection(t *testing.T) {
	master, _ := profileMetricMaster(t, "native")
	snapshot := master.live["node"]
	snapshot.report.Users.Connections = nil
	snapshot.report.Users.Rates = map[int]wire.UserRate{1: {UploadBytesPerSecond: 1}}
	master.live["node"] = snapshot
	result, err := master.UserMetrics(context.Background(), "member")
	if err != nil || result.ActivityStatus != "online" {
		t.Fatal(result, err)
	}
}
