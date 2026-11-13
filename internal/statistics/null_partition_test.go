package statistics

import (
	"context"
	"testing"
	"time"
)

func TestNullPersistedPartitionIsUnavailableAndCanRecover(t *testing.T) {
	for _, partition := range []string{remotePartition, databasePartition} {
		t.Run(partition, func(t *testing.T) {
			repository := &corruptPartitionRepository{corrupt: partition, corruptData: []byte(" \nnull\t"), saved: map[string][]byte{}}
			service := NewService(repository, &statisticsProviderStub{digest: Digest{CreatedUsers: 9}})
			before, err := service.Snapshot(context.Background())
			if err != nil || len(before.StalePartitions) != 1 || before.StalePartitions[0] != partition {
				t.Fatalf("null cache was treated as an available zero snapshot: %+v, %v", before, err)
			}
			if partition == remotePartition && (!before.RemoteGeneratedAt.IsZero() || before.DatabaseGeneratedAt.IsZero()) ||
				partition == databasePartition && (!before.DatabaseGeneratedAt.IsZero() || before.RemoteGeneratedAt.IsZero()) {
				t.Fatal("invalid cache acquired freshness or discarded its healthy sibling")
			}
			now := time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)
			if err := service.Refresh(context.Background(), now); err != nil {
				t.Fatal(err)
			}
			after, err := service.Snapshot(context.Background())
			if err != nil || len(after.StalePartitions) != 0 || !after.RemoteGeneratedAt.Equal(now) || !after.DatabaseGeneratedAt.Equal(now) || after.Remote.WeeklyUserIncrease != 9 {
				t.Fatalf("null partition blocked authoritative recovery: %+v, %v", after, err)
			}
		})
	}
}
