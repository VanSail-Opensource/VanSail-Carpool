package statistics

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type corruptPartitionRepository struct {
	statisticsRepositoryStub
	corrupt     string
	database    model.DatabaseStatistics
	databaseErr error
	saved       map[string][]byte
}

func (r *corruptPartitionRepository) LoadStatisticsPartition(_ context.Context, key string) ([]byte, time.Time, error) {
	payload := []byte("{}")
	if key == databasePartition {
		var err error
		payload, err = json.Marshal(r.database)
		if err != nil {
			return nil, time.Time{}, err
		}
	}
	if key == r.corrupt {
		payload = []byte("{invalid")
	}
	return payload, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), nil
}

func (r *corruptPartitionRepository) ProductDatabaseStatistics(context.Context, time.Time) (model.DatabaseStatistics, error) {
	return model.DatabaseStatistics{}, r.databaseErr
}

func (r *corruptPartitionRepository) SaveStatisticsPartition(_ context.Context, key string, payload []byte, _ time.Time) error {
	r.saved[key] = append([]byte(nil), payload...)
	return nil
}

func TestRefreshRepairsCorruptPersistedPartitions(t *testing.T) {
	for _, corrupt := range []string{remotePartition, databasePartition} {
		t.Run(corrupt, func(t *testing.T) {
			repository := &corruptPartitionRepository{corrupt: corrupt, saved: map[string][]byte{}}
			service := NewService(repository, &statisticsProviderStub{digest: Digest{CreatedUsers: 9}})
			now := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
			if err := service.Refresh(context.Background(), now); err != nil {
				t.Fatal(err)
			}
			snapshot, err := service.Snapshot(context.Background())
			if err != nil || snapshot.Remote.WeeklyUserIncrease != 9 || !snapshot.RemoteGeneratedAt.Equal(now) || !snapshot.DatabaseGeneratedAt.Equal(now) {
				t.Fatalf("cache did not recover: %+v, %v", snapshot, err)
			}
			for _, partition := range []string{remotePartition, databasePartition} {
				if !json.Valid(repository.saved[partition]) {
					t.Fatalf("partition %s was not repaired", partition)
				}
			}
		})
	}
}

func TestCorruptRemoteCachePreservesDatabaseThroughFailedRecompute(t *testing.T) {
	for _, initialSnapshot := range []bool{false, true} {
		name := "direct refresh"
		if initialSnapshot {
			name = "snapshot before refresh"
		}
		t.Run(name, func(t *testing.T) {
			sentinel := errors.New("database recompute unavailable")
			repository := &corruptPartitionRepository{
				corrupt: remotePartition, databaseErr: sentinel, saved: map[string][]byte{},
				database: model.DatabaseStatistics{ComboShares: []model.NamedShare{{}}},
			}
			service := NewService(repository, &statisticsProviderStub{digest: Digest{CreatedUsers: 9}})
			oldAt := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
			now := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
			if initialSnapshot {
				snapshot, err := service.Snapshot(context.Background())
				if err != nil || len(snapshot.Database.ComboShares) != 1 || !snapshot.DatabaseGeneratedAt.Equal(oldAt) || !snapshot.GeneratedAt.Equal(oldAt) || !snapshot.RemoteGeneratedAt.IsZero() || len(snapshot.StalePartitions) != 1 || snapshot.StalePartitions[0] != remotePartition {
					t.Fatalf("valid persisted database was discarded: %+v, %v", snapshot, err)
				}
			}
			if err := service.Refresh(context.Background(), now); !errors.Is(err, sentinel) {
				t.Fatalf("refresh error = %v", err)
			}
			snapshot, err := service.Snapshot(context.Background())
			if err != nil || snapshot.Remote.WeeklyUserIncrease != 9 || !snapshot.RemoteGeneratedAt.Equal(now) || len(snapshot.Database.ComboShares) != 1 || !snapshot.DatabaseGeneratedAt.Equal(oldAt) || len(snapshot.StalePartitions) != 1 || snapshot.StalePartitions[0] != databasePartition {
				t.Fatalf("last-good database did not survive: %+v, %v", snapshot, err)
			}
			if _, exists := repository.saved[databasePartition]; exists {
				t.Fatal("failed database recompute overwrote persisted last-good values")
			}
		})
	}
}

type delayedPartitionRepository struct {
	statisticsRepositoryStub
	blocked atomic.Bool
	entered chan struct{}
	release chan struct{}
	oldAt   time.Time
	failure error
}

func (r *delayedPartitionRepository) LoadStatisticsPartition(ctx context.Context, key string) ([]byte, time.Time, error) {
	if key == remotePartition && r.blocked.CompareAndSwap(false, true) {
		close(r.entered)
		select {
		case <-r.release:
		case <-ctx.Done():
			return nil, time.Time{}, ctx.Err()
		}
		if r.failure != nil {
			return nil, time.Time{}, r.failure
		}
	}
	return []byte("{}"), r.oldAt, nil
}

func TestDelayedInitialSnapshotCannotReplaceFreshPartitions(t *testing.T) {
	for _, test := range []struct {
		name    string
		failure error
	}{
		{name: "old cache values"},
		{name: "old cache error", failure: errors.New("delayed cache read failed")},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			now := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
			repository := &delayedPartitionRepository{
				entered: make(chan struct{}), release: make(chan struct{}),
				oldAt: now.Add(-time.Hour), failure: test.failure,
			}
			service := NewService(repository, &statisticsProviderStub{digest: Digest{CreatedUsers: 9}})
			type result struct {
				snapshot model.ProductStatisticsSnapshot
				err      error
			}
			done := make(chan result, 1)
			go func() {
				snapshot, err := service.Snapshot(ctx)
				done <- result{snapshot, err}
			}()
			select {
			case <-repository.entered:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			if err := service.Refresh(ctx, now); err != nil {
				t.Fatal(err)
			}
			close(repository.release)
			select {
			case got := <-done:
				if got.err != nil || got.snapshot.Remote.WeeklyUserIncrease != 9 || !got.snapshot.RemoteGeneratedAt.Equal(now) || !got.snapshot.DatabaseGeneratedAt.Equal(now) {
					t.Fatalf("late hydration replaced fresh state: %+v, %v", got.snapshot, got.err)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		})
	}
}
