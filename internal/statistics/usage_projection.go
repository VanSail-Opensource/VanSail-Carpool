package statistics

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usageaccounting"
)

type usageProjectionStatistics struct{ monthlyAverageUsageBPS int }

func (s *Service) usageProjectionStatistics(ctx context.Context, now time.Time) (usageProjectionStatistics, error) {
	members, err := s.repository.ActiveMemberUsageForStatistics(ctx, now)
	if err != nil {
		return usageProjectionStatistics{}, fmt.Errorf("list active member usage: %w", err)
	}
	total := 0.0
	count := 0
	for _, member := range members {
		end := now
		if end.After(member.Purchase.ValidUntil) {
			end = member.Purchase.ValidUntil
		}
		if !end.After(member.Purchase.ValidFrom) {
			continue
		}
		snapshot, err := s.provider.UsageSnapshot(ctx, member.RemoteUserID, member.Purchase.ValidFrom, end)
		if err != nil || !snapshot.NodeSeriesAvailable {
			continue
		}
		allocated := usageaccounting.CalculateUsage(member.Purchase, snapshot).AllocatedBytes
		if allocated <= 0 || snapshot.WeightedUsedBytes < 0 {
			continue
		}
		total += float64(snapshot.WeightedUsedBytes) * 10000 / float64(allocated)
		count++
	}
	result := usageProjectionStatistics{}
	if count > 0 {
		result.monthlyAverageUsageBPS = int(math.Round(total / float64(count)))
	}
	return result, nil
}
