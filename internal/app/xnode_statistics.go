package app

import (
	"context"
	stats "github.com/VanSail-Opensource/VanSail-Carpool/internal/statistics"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"math"
	"time"
)

func (a nativeStatisticsProvider) nativeTraffic(ctx context.Context, from, to time.Time) (stats.Traffic, error) {
	result := stats.Traffic{Categories: []string{}, Series: []stats.TrafficSeries{}}
	usage, err := a.store.XNodeFleetUsage(ctx, from, to)
	if err != nil {
		return result, err
	}
	byNode := map[string]map[string]int64{}
	for _, sample := range usage {
		if byNode[sample.NodeID] == nil {
			byNode[sample.NodeID] = map[string]int64{}
		}
		if sample.UploadBytes > math.MaxInt64-sample.DownloadBytes {
			return result, xnode.ErrInvalid
		}
		bytes := sample.UploadBytes + sample.DownloadBytes
		previous := byNode[sample.NodeID][sample.Date]
		if previous > math.MaxInt64-bytes {
			return result, xnode.ErrInvalid
		}
		byNode[sample.NodeID][sample.Date] = previous + bytes
	}
	for day := from.UTC(); !day.After(to); day = day.AddDate(0, 0, 1) {
		result.Categories = append(result.Categories, day.Format(time.DateOnly))
	}
	nodes, err := a.native.Nodes(ctx)
	if err != nil {
		return result, err
	}
	for _, node := range nodes {
		series := stats.TrafficSeries{UUID: node.ID, Name: node.Name, CountryCode: node.CountryCode, DailyBytes: []int64{}}
		for _, date := range result.Categories {
			series.DailyBytes = append(series.DailyBytes, byNode[node.ID][date])
		}
		result.Series = append(result.Series, series)
	}
	return result, nil
}
