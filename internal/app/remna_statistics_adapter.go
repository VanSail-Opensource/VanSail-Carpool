package app

import (
	"context"
	"math"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	productstats "github.com/VanSail-Opensource/VanSail-Carpool/internal/statistics"
)

func (a remnaAdapter) Digest(ctx context.Context, start, end time.Time) (productstats.Digest, error) {
	digest, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) (remnawave.StatsDigest, error) {
		return client.GetStatsDigest(callCtx, start, end)
	})
	return productstats.Digest{CreatedUsers: digest.CreatedUsers, ExpiredUsers: digest.ExpiredUsers}, err
}

func (a remnaAdapter) Traffic(ctx context.Context, start, end time.Time) (productstats.Traffic, error) {
	usage, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) (remnawave.NodesUsage, error) {
		return client.GetNodesUsage(callCtx, start, end)
	})
	if err != nil {
		return productstats.Traffic{}, err
	}
	result := productstats.Traffic{Categories: append([]string(nil), usage.Categories...), Series: make([]productstats.TrafficSeries, 0, len(usage.Series))}
	for _, series := range usage.Series {
		result.Series = append(result.Series, productstats.TrafficSeries{UUID: series.UUID, Name: series.Name,
			CountryCode: series.CountryCode, DailyBytes: append([]int64(nil), series.Data...)})
	}
	return result, nil
}

func (a remnaAdapter) SquadNames(ctx context.Context) (map[string]string, error) {
	squads, err := a.listSquads(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(squads))
	for _, squad := range squads {
		result[squad.UUID] = squad.Name
	}
	return result, nil
}

func (a remnaAdapter) Nodes(ctx context.Context) ([]productstats.Node, error) {
	// The documented node collection already carries the live health and rate fields.
	nodes, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) ([]remnawave.Node, error) {
		return client.ListNodes(callCtx)
	})
	if err != nil {
		return nil, err
	}
	result := make([]productstats.Node, 0, len(nodes))
	for _, node := range nodes {
		item := productstats.Node{UUID: node.UUID, Name: node.Name, CountryCode: node.CountryCode,
			Online: node.IsConnected && !node.IsDisabled, UsersOnline: roundedNonNegativeInt64(node.UsersOnline), Multiplier: node.ConsumptionMultiplier}
		if node.System != nil && node.System.Stats.Interface != nil {
			item.RXBytesPerSecond = roundedNonNegativeInt64(node.System.Stats.Interface.RXBytesPerSecond)
			item.TXBytesPerSecond = roundedNonNegativeInt64(node.System.Stats.Interface.TXBytesPerSecond)
		}
		if node.Versions != nil {
			item.XrayVersion = node.Versions.Xray
		}
		result = append(result, item)
	}
	return result, nil
}

func roundedNonNegativeInt64(value float64) int64 {
	if value <= 0 || math.IsNaN(value) {
		return 0
	}
	const maximum = int64(1<<63 - 1)
	if value >= float64(maximum) {
		return maximum
	}
	return int64(math.Round(value))
}

func (a remnaAdapter) RequestNodeGeocheck(ctx context.Context, nodeUUID string) (string, error) {
	return remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) (string, error) {
		return client.RequestNodeGeocheck(callCtx, nodeUUID)
	})
}

func (a remnaAdapter) NodeGeocheckResult(ctx context.Context, jobID string) (productstats.GeocheckResult, error) {
	result, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) (remnawave.NodeGeocheck, error) {
		return client.NodeGeocheckResult(callCtx, jobID)
	})
	if err != nil {
		return productstats.GeocheckResult{}, err
	}
	mapped := productstats.GeocheckResult{Completed: result.Completed, Failed: result.Failed, Success: result.Success, NodeUUID: result.NodeUUID}
	if result.Image != nil {
		mapped.Image = &productstats.GeocheckImage{Format: result.Image.Format, MediaType: result.Image.MediaType,
			Encoding: result.Image.Encoding, Data: result.Image.Data}
	}
	return mapped, nil
}

func (a remnaAdapter) Hosts(ctx context.Context) ([]productstats.Host, error) {
	hosts, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) ([]remnawave.Host, error) {
		return client.ListHosts(callCtx)
	})
	result := make([]productstats.Host, 0, len(hosts))
	for _, host := range hosts {
		result = append(result, productstats.Host{UUID: host.UUID, Remark: host.Remark, Nodes: append([]string(nil), host.Nodes...)})
	}
	return result, err
}

func (a remnaAdapter) UpdateHostRemark(ctx context.Context, hostUUID, remark string) error {
	return remnaExecute(ctx, a, func(callCtx context.Context, client remnaClient) error {
		return client.UpdateHostRemark(callCtx, hostUUID, remark)
	})
}

var _ productstats.Provider = remnaAdapter{}
var _ productstats.SquadNameProvider = remnaAdapter{}
