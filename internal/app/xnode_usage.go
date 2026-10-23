package app

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"strconv"
	"time"
)

func (a proxyProvider) NodeUsage(ctx context.Context, id string, start, end time.Time) (model.DashboardNodeUsage, error) {
	_, err := a.native.Identity(ctx, id)
	if err != nil {
		return model.DashboardNodeUsage{}, err
	}
	usage, err := a.native.Usage(ctx, id, start, end)
	if err != nil {
		return model.DashboardNodeUsage{}, err
	}
	result := model.DashboardNodeUsage{Categories: []string{}, Nodes: []model.DashboardNodeUsageNode{}}
	dates := map[string]int{}
	byNode := map[string]map[string]int64{}
	for day := start.UTC(); !day.After(end); day = day.AddDate(0, 0, 1) {
		date := day.Format(time.DateOnly)
		dates[date] = len(result.Categories)
		result.Categories = append(result.Categories, date)
	}
	for _, sample := range usage {
		if byNode[sample.NodeID] == nil {
			byNode[sample.NodeID] = map[string]int64{}
		}
		byNode[sample.NodeID][sample.Date] += sample.UploadBytes + sample.DownloadBytes
	}
	nodes, err := a.native.Nodes(ctx)
	if err != nil {
		return result, err
	}
	for _, node := range nodes {
		item := model.DashboardNodeUsageNode{UUID: node.ID, Name: node.Name, CountryCode: node.CountryCode, DailyBytes: []string{}}
		var total int64
		for _, date := range result.Categories {
			value := byNode[node.ID][date]
			total += value
			item.DailyBytes = append(item.DailyBytes, strconv.FormatInt(value, 10))
		}
		item.TotalBytes = strconv.FormatInt(total, 10)
		result.Nodes = append(result.Nodes, item)
	}
	return result, nil
}
