package app

import (
	"context"
	stats "github.com/VanSail-Opensource/VanSail-Carpool/internal/statistics"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"math"
	"time"
)

func (a proxyProvider) Digest(ctx context.Context, from, to time.Time) (stats.Digest, error) {
	nodes, err := a.native.Nodes(ctx)
	if err != nil {
		return stats.Digest{}, err
	}
	if len(nodes) == 0 {
		bridge, err := a.legacy(ctx)
		if err != nil {
			return stats.Digest{}, err
		}
		if bridge {
			return a.remnaAdapter.Digest(ctx, from, to)
		}
	}
	count, err := a.store.XNodeIdentityGrowth(ctx, from, to)
	return stats.Digest{CreatedUsers: count}, err
}
func (a proxyProvider) Traffic(ctx context.Context, from, to time.Time) (stats.Traffic, error) {
	result := stats.Traffic{Categories: []string{}, Series: []stats.TrafficSeries{}}
	nativeNodes, nodeErr := a.native.Nodes(ctx)
	if nodeErr != nil {
		return result, nodeErr
	}
	if len(nativeNodes) == 0 {
		bridge, err := a.legacy(ctx)
		if err != nil {
			return result, err
		}
		if bridge {
			return a.remnaAdapter.Traffic(ctx, from, to)
		}
	}
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
func (a proxyProvider) Nodes(ctx context.Context) ([]stats.Node, error) {
	nodes, err := a.native.Nodes(ctx)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		bridge, err := a.legacy(ctx)
		if err != nil {
			return nil, err
		}
		if bridge {
			return a.remnaAdapter.Nodes(ctx)
		}
	}
	metrics, err := a.master.Metrics(ctx)
	if err != nil {
		return nil, err
	}
	live := map[string]xnode.NodeMetrics{}
	for _, node := range metrics.Nodes {
		live[node.NodeID] = node
	}
	out := []stats.Node{}
	for _, node := range nodes {
		current := live[node.ID]
		out = append(out, stats.Node{UUID: node.ID, Name: node.Name, CountryCode: node.CountryCode, Multiplier: node.Multiplier, Online: !current.Stale && current.Ready && node.Enabled, UsersOnline: current.UsersOnline, RXBytesPerSecond: roundedNonNegativeInt64(current.DownloadBytesPerSecond), TXBytesPerSecond: roundedNonNegativeInt64(current.UploadBytesPerSecond), XrayVersion: current.XrayVersion})
	}
	bridge, err := a.legacy(ctx)
	if err != nil {
		return nil, err
	}
	if bridge {
		legacy, err := a.remnaAdapter.Nodes(ctx)
		if err != nil {
			return out, nil
		}
		for _, remote := range legacy {
			owned, err := a.native.NativeOwner(ctx, remote.UUID)
			if err != nil {
				return nil, err
			}
			if owned {
				continue
			}
			for i := range out {
				if out[i].UUID == remote.UUID {
					out[i] = remote
					break
				}
			}
		}
	}
	return out, nil
}
