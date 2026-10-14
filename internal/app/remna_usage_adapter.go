package app

import (
	"context"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const dashboardUsageTopNodesLimit = 20

// NodeUsage maps the documented Remnawave user usage series into the
// authenticated member projection. remnaCall keeps this live read inside the
// shared provider queue.
func (a remnaAdapter) NodeUsage(ctx context.Context, remoteID string, start, end time.Time) (model.DashboardNodeUsage, error) {
	userID, err := remnaUserID(remoteID)
	if err != nil {
		return model.DashboardNodeUsage{}, err
	}
	stats, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) (*remnawave.UserStats, error) {
		return client.GetUserStats(callCtx, userID, start, end, dashboardUsageTopNodesLimit)
	})
	if err != nil {
		return model.DashboardNodeUsage{}, err
	}
	report := model.DashboardNodeUsage{
		Categories: append([]string(nil), stats.Categories...),
		Nodes:      make([]model.DashboardNodeUsageNode, 0, len(stats.Series)),
	}
	for _, series := range stats.Series {
		if len(report.Nodes) == dashboardUsageTopNodesLimit {
			break
		}
		dailyBytes := make([]string, 0, len(series.Data))
		for _, value := range series.Data {
			dailyBytes = append(dailyBytes, strconv.FormatInt(value, 10))
		}
		report.Nodes = append(report.Nodes, model.DashboardNodeUsageNode{
			UUID: series.UUID, Name: series.Name, CountryCode: series.CountryCode,
			TotalBytes: strconv.FormatInt(series.Total, 10), DailyBytes: dailyBytes,
		})
	}
	return report, nil
}
