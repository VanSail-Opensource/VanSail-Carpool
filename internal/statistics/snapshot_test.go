package statistics

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestSnapshotSerializesContractArrays(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	service := &Service{snapshot: model.ProductStatisticsSnapshot{
		RemoteGeneratedAt: now,
		Remote:            model.RemoteStatistics{TrafficSeries: []model.NodeTrafficSeries{{}}},
		Database: model.DatabaseStatistics{
			SquadByCombo: []model.NormalizedDistribution{{}},
			ComboBySquad: []model.NormalizedDistribution{{}},
		},
	}}

	snapshot, err := service.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot(): %v", err)
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("json.Marshal(): %v", err)
	}
	if bytes.Contains(payload, []byte(":null")) {
		t.Fatalf("snapshot contains a null contract array: %s", payload)
	}
}
