package statistics

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
)

type usageProjectionStatistics struct {
	monthlyAverageUsageBPS   int
	predictedAverageRollover model.Money
}

func (s *Service) usageProjectionStatistics(ctx context.Context, now time.Time) (usageProjectionStatistics, error) {
	members, err := s.repository.ActiveMemberUsageForStatistics(ctx, now)
	if err != nil {
		return usageProjectionStatistics{}, fmt.Errorf("list active members for usage statistics: %w", err)
	}

	var usageTotal float64
	usageCount := 0
	rolloverTotal := new(big.Int)
	var rolloverCount int64
	for _, member := range members {
		purchase := member.Purchase
		start := purchase.ValidFrom.UTC()
		end := now
		if end.After(purchase.ValidUntil) {
			end = purchase.ValidUntil.UTC()
		}
		if !end.After(start) {
			continue
		}
		snapshot, snapshotErr := s.provider.UsageSnapshotForRollover(ctx, member.RemoteUserID, start, end)
		if errors.Is(snapshotErr, rollover.ErrRemoteUserMissing) {
			continue
		}
		if snapshotErr != nil {
			continue
		}
		projection := rollover.ProjectUsage(purchase, purchase.RolloverMinRemainingBPS, snapshot, now)
		if projection.Term == nil || projection.ActualUsedTrafficBytes == nil || projection.Term.AllocatedTrafficBytes <= 0 {
			continue
		}
		ratio := float64(*projection.ActualUsedTrafficBytes) * 10000 / float64(projection.Term.AllocatedTrafficBytes)
		if ratio < 0 || math.IsNaN(ratio) || math.IsInf(ratio, 0) {
			continue
		}
		usageTotal += ratio
		usageCount++
		if !purchase.AutoRenewEnabled {
			continue
		}
		rolloverCount++
		if projection.PredictedRollover != nil {
			rolloverTotal.Add(rolloverTotal, big.NewInt(projection.PredictedRollover.MinorInt64()))
		}
	}
	result := usageProjectionStatistics{predictedAverageRollover: model.TXBMoney(roundedAverageMinor(rolloverTotal, rolloverCount))}
	if usageCount > 0 {
		result.monthlyAverageUsageBPS = int(math.Round(usageTotal / float64(usageCount)))
	}
	return result, nil
}

func roundedAverageMinor(total *big.Int, count int64) int64 {
	if total == nil || total.Sign() <= 0 || count <= 0 {
		return 0
	}
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(total, big.NewInt(count), remainder)
	if remainder.Cmp(big.NewInt((count+1)/2)) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return quotient.Int64()
}
