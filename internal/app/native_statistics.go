package app

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	stats "github.com/VanSail-Opensource/VanSail-Carpool/internal/statistics"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usageaccounting"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// nativeStatisticsProvider reads only panel accounting and native memory snapshots.
// Commerce/compensation keep their existing queued migration bridge provider.
type nativeStatisticsProvider struct{ proxyProvider }

func (a nativeStatisticsProvider) Digest(ctx context.Context, from, to time.Time) (stats.Digest, error) {
	count, err := a.store.XNodeIdentityGrowth(ctx, from, to)
	return stats.Digest{CreatedUsers: count}, err
}

func (a nativeStatisticsProvider) Traffic(ctx context.Context, from, to time.Time) (stats.Traffic, error) {
	return a.nativeTraffic(ctx, from, to)
}

func (a nativeStatisticsProvider) Nodes(ctx context.Context) ([]stats.Node, error) {
	nodes, err := a.native.Nodes(ctx)
	if err != nil {
		return nil, err
	}
	metrics, err := a.master.Metrics(ctx)
	if err != nil {
		return nil, err
	}
	live := map[string]xnode.NodeMetrics{}
	for _, node := range metrics.Nodes {
		live[node.NodeID] = node
	}
	result := []stats.Node{}
	for _, node := range nodes {
		current := live[node.ID]
		owned, err := a.native.NativeOwner(ctx, node.ID)
		if err != nil {
			return nil, err
		}
		available := owned && node.Enabled && current.Connected && current.Ready && current.Synchronized && !current.Stale && slices.Contains(current.Capabilities, "sampled_activity_v1")
		item := stats.Node{UUID: node.ID, Name: node.Name, CountryCode: node.CountryCode, Multiplier: node.Multiplier, Online: available, XrayVersion: current.XrayVersion}
		if available {
			item.UsersOnline = current.UsersOnline
			item.RXBytesPerSecond = roundedNonNegativeInt64(current.DownloadBytesPerSecond)
			item.TXBytesPerSecond = roundedNonNegativeInt64(current.UploadBytesPerSecond)
		}
		result = append(result, item)
	}
	return result, nil
}

func (a nativeStatisticsProvider) SquadNames(ctx context.Context) (map[string]string, error) {
	groups, err := a.native.Groups(ctx)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, group := range groups {
		result[group.ID] = group.Name
	}
	return result, nil
}

func (a nativeStatisticsProvider) UsageSnapshot(ctx context.Context, id string, from, to time.Time) (usageaccounting.UsageSnapshot, error) {
	// The statistics repository supplies native proxy IDs whenever available.
	var ok bool
	id, ok = strings.CutPrefix(id, "xn:")
	if !ok {
		return usageaccounting.UsageSnapshot{}, xnode.ErrNotFound
	}
	if _, err := strconv.ParseInt(id, 10, 53); err != nil {
		return usageaccounting.UsageSnapshot{}, xnode.ErrNotFound
	}
	return a.native.UsageSnapshot(ctx, id, from, to)
}
