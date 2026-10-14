package xnode

import (
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestFreshSnapshotsExpireAndMissingCountsAreNotMerged(t *testing.T) {
	now := time.Now()
	snapshot := liveSnapshot{receivedAt: now.Add(-14 * time.Second)}
	if !freshSnapshot(snapshot, now, 5) || freshSnapshot(snapshot, now.Add(2*time.Second), 5) {
		t.Fatal("freshness boundary is incorrect")
	}
	current := wire.UserMetrics{Connections: map[int]int{7: 2}, Status: map[int]wire.UserLimitStatus{}, Rates: map[int]wire.UserRate{}}
	if !validMetrics(current) {
		t.Fatal("valid snapshot rejected")
	}
	current.Connections[7] = -1
	if validMetrics(current) {
		t.Fatal("negative count accepted")
	}
	current.Connections = map[int]int{}
	if !validMetrics(current) {
		t.Fatal("authoritative empty snapshot rejected")
	}
}
