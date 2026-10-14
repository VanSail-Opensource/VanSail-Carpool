package app

import (
	"context"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/catalog"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (a remnaAdapter) Dashboard(ctx context.Context, remoteID string) (catalog.RemoteDashboard, error) {
	userID, err := remnaUserID(remoteID)
	if err != nil {
		return catalog.RemoteDashboard{}, err
	}
	user, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) (*remnawave.User, error) {
		return client.GetUserByID(callCtx, userID)
	})
	if err != nil {
		return catalog.RemoteDashboard{}, err
	}
	now := time.Now().UTC()
	stats, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) (*remnawave.UserStats, error) {
		return client.GetUserStats(callCtx, userID, now.AddDate(0, -1, 0), now, 5)
	})
	if err != nil {
		return catalog.RemoteDashboard{}, err
	}
	multipliers := make(map[string]float64)
	nodes, nodeErr := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) ([]remnawave.Node, error) {
		return client.ListNodes(callCtx)
	})
	if nodeErr == nil {
		for _, node := range nodes {
			multipliers[node.UUID] = node.ConsumptionMultiplier
		}
	}
	mapped := model.Statistics{
		UsedTrafficBytes:     strconv.FormatInt(user.UserTraffic.UsedTrafficBytes, 10),
		LifetimeTrafficBytes: strconv.FormatInt(user.UserTraffic.LifetimeUsedTrafficBytes, 10),
		TrafficLimitBytes:    strconv.FormatInt(user.TrafficLimitBytes, 10), OnlineAt: user.UserTraffic.OnlineAt,
		LastTrafficResetAt: user.LastTrafficResetAt,
		Categories:         stats.Categories, SparklineData: make([]string, 0, len(stats.SparklineData)),
		TopNodes: make([]model.TopNode, 0, len(stats.TopNodes)),
	}
	for _, sample := range stats.SparklineData {
		mapped.SparklineData = append(mapped.SparklineData, strconv.FormatInt(sample, 10))
	}
	for _, node := range stats.TopNodes {
		item := model.TopNode{
			UUID: node.UUID, Name: node.Name, CountryCode: node.CountryCode,
			TotalBytes: strconv.FormatInt(node.Total, 10),
		}
		if multiplier, ok := multipliers[node.UUID]; ok {
			item.ConsumptionMultiplier = &multiplier
		}
		mapped.TopNodes = append(mapped.TopNodes, item)
	}
	return catalog.RemoteDashboard{Statistics: mapped, SubscriptionURL: user.SubscriptionURL}, nil
}

func (a remnaAdapter) RevokeSubscription(ctx context.Context, remoteID string) (string, error) {
	userID, err := remnaUserID(remoteID)
	if err != nil {
		return "", err
	}
	user, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) (*remnawave.User, error) {
		return client.RevokeSubscription(callCtx, userID, false)
	})
	if err != nil {
		return "", err
	}
	return user.SubscriptionURL, nil
}

var _ catalog.RemnawaveClient = remnaAdapter{}
