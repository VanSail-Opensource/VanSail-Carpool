package statistics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const (
	remotePartition   = "remote"
	databasePartition = "database"
)

// Repository calculates local facts and persists independent last-good parts.
type Repository interface {
	ProductDatabaseStatistics(context.Context, time.Time) (model.DatabaseStatistics, error)
	ActiveMemberUsageForStatistics(context.Context, time.Time) ([]model.StatisticsUsageMember, error)
	LoadStatisticsPartition(context.Context, string) ([]byte, time.Time, error)
	SaveStatisticsPartition(context.Context, string, []byte, time.Time) error
}

// Service owns refresh serialization and last-good aggregate state.
type Service struct {
	repository Repository
	provider   Provider
	mu         sync.RWMutex
	refreshMu  sync.Mutex
	nodeMu     sync.Mutex
	snapshot   model.ProductStatisticsSnapshot
	nodes      nodeCache
	geocheck   geocheckCache
}

// NewService creates the statistics cache.
func NewService(repository Repository, provider Provider) *Service {
	return &Service{repository: repository, provider: provider}
}

// Refresh updates each partition independently and preserves failed parts.
func (s *Service) Refresh(ctx context.Context, now time.Time) error {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	now = normalizedNow(now)
	if err := s.loadLastGood(ctx); err != nil {
		return err
	}
	remote, remoteErr := s.refreshRemote(ctx, now)
	if remoteErr == nil {
		remoteErr = s.saveRemote(ctx, remote, now)
	}
	squadNames := s.resolveSquadNames(ctx)
	database, databaseErr := s.repository.ProductDatabaseStatistics(ctx, now)
	if databaseErr == nil {
		applySquadNames(&database, squadNames)
		databaseErr = s.saveDatabase(ctx, database, now)
	}
	s.mu.Lock()
	s.snapshot.GeneratedAt = now
	s.snapshot.StalePartitions = stalePartitions(remoteErr, databaseErr)
	s.mu.Unlock()
	return errors.Join(remoteErr, databaseErr)
}

// Snapshot returns an immutable copy of the current last-good state.
func (s *Service) Snapshot(ctx context.Context) (model.ProductStatisticsSnapshot, error) {
	if err := s.loadLastGood(ctx); err != nil {
		return model.ProductStatisticsSnapshot{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneSnapshot(s.snapshot), nil
}

func cloneSnapshot(value model.ProductStatisticsSnapshot) model.ProductStatisticsSnapshot {
	result := value
	result.StalePartitions = append([]string{}, value.StalePartitions...)
	result.Remote.TrafficDates = append([]string{}, value.Remote.TrafficDates...)
	result.Remote.TrafficSeries = append([]model.NodeTrafficSeries{}, value.Remote.TrafficSeries...)
	for index := range result.Remote.TrafficSeries {
		result.Remote.TrafficSeries[index].DailyBytes = append([]string{}, value.Remote.TrafficSeries[index].DailyBytes...)
	}
	result.Database.SubscriptionStates = append([]model.NamedShare{}, value.Database.SubscriptionStates...)
	result.Database.ComboShares = append([]model.NamedShare{}, value.Database.ComboShares...)
	result.Database.PaymentStatuses = append([]model.NamedShare{}, value.Database.PaymentStatuses...)
	result.Database.SquadByCombo = cloneDistributions(value.Database.SquadByCombo)
	result.Database.ComboBySquad = cloneDistributions(value.Database.ComboBySquad)
	return result
}

func cloneDistributions(values []model.NormalizedDistribution) []model.NormalizedDistribution {
	result := append([]model.NormalizedDistribution{}, values...)
	for index := range result {
		result[index].Segments = append([]model.NamedShare{}, values[index].Segments...)
	}
	return result
}

func (s *Service) refreshRemote(ctx context.Context, now time.Time) (model.RemoteStatistics, error) {
	digest, err := s.provider.Digest(ctx, now.AddDate(0, 0, -7), now)
	if err != nil {
		return model.RemoteStatistics{}, err
	}
	start := time.Date(now.Year(), now.Month(), now.Day()-6, 0, 0, 0, 0, time.UTC)
	traffic, err := s.provider.Traffic(ctx, start, now)
	if err != nil {
		return model.RemoteStatistics{}, err
	}
	usage, err := s.usageProjectionStatistics(ctx, now)
	if err != nil {
		return model.RemoteStatistics{}, err
	}
	result := model.RemoteStatistics{WeeklyUserIncrease: digest.CreatedUsers,
		MonthlyAverageUsageBPS: usage.monthlyAverageUsageBPS, MonthlyAverageUsage: float64(usage.monthlyAverageUsageBPS) / 100,
		PredictedAverageRollover: usage.predictedAverageRollover,
		TrafficDates:             append([]string(nil), traffic.Categories...), TrafficSeries: make([]model.NodeTrafficSeries, 0, len(traffic.Series))}
	for _, series := range traffic.Series {
		values := make([]string, 0, len(series.DailyBytes))
		for _, value := range series.DailyBytes {
			values = append(values, strconv.FormatInt(value, 10))
		}
		result.TrafficSeries = append(result.TrafficSeries, model.NodeTrafficSeries{UUID: series.UUID, Name: series.Name, CountryCode: series.CountryCode, DailyBytes: values})
	}
	return result, nil
}

func normalizedNow(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}

func stalePartitions(remoteErr, databaseErr error) []string {
	result := make([]string, 0, 2)
	if remoteErr != nil {
		result = append(result, remotePartition)
	}
	if databaseErr != nil {
		result = append(result, databasePartition)
	}
	return result
}

// ErrPartitionNotFound is returned by repositories before the first refresh.
var ErrPartitionNotFound = errors.New("statistics partition is unavailable")

func decodePartition[T any](payload []byte) (T, error) {
	var result T
	if len(payload) == 0 || json.Unmarshal(payload, &result) != nil {
		return result, fmt.Errorf("%w: invalid partition", ErrPartitionNotFound)
	}
	return result, nil
}
