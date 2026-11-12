package app

import (
	"context"
	"math/big"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// NodeUsageForMember resolves the panel owner, never a historical proxy pointer.
func (a proxyProvider) NodeUsageForMember(ctx context.Context, user model.User, start, end time.Time) (model.DashboardNodeUsage, error) {
	if user.ID == "" {
		return model.DashboardNodeUsage{}, xnode.ErrInvalid
	}
	identity, err := a.native.IdentityForUser(ctx, user.ID)
	if err != nil {
		return model.DashboardNodeUsage{}, err
	}
	if identity.UserID != user.ID {
		return model.DashboardNodeUsage{}, xnode.ErrConflict
	}
	usage, err := a.native.Usage(ctx, strconv.FormatInt(identity.ProxyUserID, 10), start, end)
	if err != nil {
		return model.DashboardNodeUsage{}, err
	}
	result := model.DashboardNodeUsage{Categories: []string{}, Nodes: []model.DashboardNodeUsageNode{}}
	byNode := map[string]map[string]*big.Int{}
	for day := start.UTC(); !day.After(end); day = day.AddDate(0, 0, 1) {
		date := day.Format(time.DateOnly)
		result.Categories = append(result.Categories, date)
	}
	for _, sample := range usage {
		if sample.UploadBytes < 0 || sample.DownloadBytes < 0 {
			return model.DashboardNodeUsage{}, xnode.ErrInvalid
		}
		if byNode[sample.NodeID] == nil {
			byNode[sample.NodeID] = map[string]*big.Int{}
		}
		if byNode[sample.NodeID][sample.Date] == nil {
			byNode[sample.NodeID][sample.Date] = new(big.Int)
		}
		value := byNode[sample.NodeID][sample.Date]
		value.Add(value, big.NewInt(sample.UploadBytes))
		value.Add(value, big.NewInt(sample.DownloadBytes))
	}
	nodes, err := a.native.Nodes(ctx)
	if err != nil {
		return result, err
	}
	for _, node := range nodes {
		item := model.DashboardNodeUsageNode{UUID: node.ID, Name: node.Name, CountryCode: node.CountryCode, DailyBytes: []string{}}
		total := new(big.Int)
		for _, date := range result.Categories {
			value := byNode[node.ID][date]
			if value == nil {
				item.DailyBytes = append(item.DailyBytes, "0")
				continue
			}
			total.Add(total, value)
			item.DailyBytes = append(item.DailyBytes, value.String())
		}
		item.TotalBytes = total.String()
		result.Nodes = append(result.Nodes, item)
	}
	return result, nil
}
