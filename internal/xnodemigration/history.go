package xnodemigration

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"math"
	"math/big"
	"time"
)

// History requires all node series to reconcile with the source's raw daily
// aggregates. Missing series never turn into invented unused-traffic credit.
func History(user int64, stats remnawave.UserStats, multipliers map[string]int64) ([]xnode.LegacyHistory, error) {
	if len(stats.Categories) != len(stats.SparklineData) || stats.Series == nil {
		return nil, xnode.ErrUnavailable
	}
	seenDays := map[string]bool{}
	for i, day := range stats.Categories {
		if _, err := time.Parse(time.DateOnly, day); err != nil || seenDays[day] || stats.SparklineData[i] < 0 {
			return nil, xnode.ErrInvalid
		}
		seenDays[day] = true
	}
	seenNodes := map[string]bool{}
	totals := make([]int64, len(stats.Categories))
	result := []xnode.LegacyHistory{}
	for _, series := range stats.Series {
		fp := multipliers[series.UUID]
		if seenNodes[series.UUID] || fp <= 0 || len(series.Data) != len(totals) {
			return nil, xnode.ErrUnavailable
		}
		seenNodes[series.UUID] = true
		for i, raw := range series.Data {
			if raw < 0 || totals[i] > math.MaxInt64-raw {
				return nil, xnode.ErrInvalid
			}
			totals[i] += raw
			weighted := new(big.Int).Mul(big.NewInt(raw), big.NewInt(fp))
			weighted.Quo(weighted, big.NewInt(xnode.MultiplierScale))
			if !weighted.IsInt64() {
				return nil, xnode.ErrInvalid
			}
			result = append(result, xnode.LegacyHistory{ProxyUserID: user, NodeID: series.UUID, Date: stats.Categories[i], RawBytes: raw, WeightedBytes: weighted.Int64()})
		}
	}
	for i, total := range totals {
		if total != stats.SparklineData[i] {
			return nil, xnode.ErrUnavailable
		}
	}
	return result, nil
}

func (s *Service) importHistory(ctx context.Context, identity xnode.Identity, account Account, snapshot Snapshot) error {
	multipliers := map[string]int64{}
	for _, node := range snapshot.Nodes {
		multipliers[node.ID] = node.MultiplierFP
	}
	values, err := History(identity.ProxyUserID, account.Stats, multipliers)
	if err != nil {
		return err
	}
	return s.repo.SaveXNodeLegacyHistory(ctx, values)
}
