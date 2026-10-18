package xnode

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"math/big"
	"strconv"
	"time"
)

func (s *Service) Statistics(ctx context.Context, id string) (model.Statistics, string, error) {
	result, identity, err := s.accountStatistics(ctx, id)
	if err != nil {
		return result, "", err
	}
	link, err := s.SubscriptionURL(identity)
	return result, link, err
}
func (s *Service) accountStatistics(ctx context.Context, id string) (model.Statistics, Identity, error) {
	identity, err := s.Identity(ctx, id)
	if err != nil {
		return model.Statistics{}, Identity{}, err
	}
	result := model.Statistics{UsedTrafficBytes: "0", LifetimeTrafficBytes: "0", TrafficLimitBytes: "0", Categories: []string{}, SparklineData: []string{}, TopNodes: []model.TopNode{}}
	if identity.CurrentPeriodID != "" {
		period, err := s.repo.XNodePeriod(ctx, identity.CurrentPeriodID)
		if err != nil {
			return result, Identity{}, err
		}
		result.UsedTrafficBytes = strconv.FormatInt(period.UsedBytes, 10)
		result.TrafficLimitBytes = strconv.FormatInt(period.LimitBytes, 10)
		result.LastTrafficResetAt = period.LastResetAt
	}
	periods, err := s.repo.XNodePeriods(ctx, identity.ProxyUserID)
	if err != nil {
		return result, Identity{}, err
	}
	lifetime := big.NewInt(identity.LifetimeBase)
	for _, period := range periods {
		lifetime.Add(lifetime, big.NewInt(period.UsedBytes))
	}
	legacy, legacyErr := s.repo.XNodeLegacyUsage(ctx, identity.ProxyUserID)
	if legacyErr == nil {
		lifetime = big.NewInt(legacy.LifetimeBytes)
		all, err := s.repo.XNodeUsage(ctx, identity.ProxyUserID, time.Unix(0, 0), s.now())
		if err != nil {
			return result, Identity{}, err
		}
		for _, sample := range all {
			lifetime.Add(lifetime, big.NewInt(sample.WeightedBytes))
		}
	} else if !isMissing(legacyErr) {
		return result, Identity{}, legacyErr
	}
	result.LifetimeTrafficBytes = lifetime.String()
	now := s.now().UTC()
	usage, err := s.repo.XNodeUsage(ctx, identity.ProxyUserID, now.AddDate(0, -1, 0), now)
	if err != nil {
		return result, Identity{}, err
	}
	legacyHistory, err := s.repo.XNodeLegacyHistory(ctx, identity.ProxyUserID, now.AddDate(0, -1, 0), now)
	if err != nil {
		return result, Identity{}, err
	}
	for _, value := range legacyHistory {
		usage = append(usage, DailyUsage{ProxyUserID: value.ProxyUserID, NodeID: value.NodeID, Date: value.Date, UploadBytes: value.RawBytes, WeightedBytes: value.WeightedBytes})
	}
	daily := map[string]*big.Int{}
	totals := map[string]*big.Int{}
	for _, item := range usage {
		if item.WeightedBytes < 0 {
			return result, Identity{}, ErrInvalid
		}
		if daily[item.Date] == nil {
			daily[item.Date] = new(big.Int)
		}
		if totals[item.NodeID] == nil {
			totals[item.NodeID] = new(big.Int)
		}
		daily[item.Date].Add(daily[item.Date], big.NewInt(item.WeightedBytes))
		totals[item.NodeID].Add(totals[item.NodeID], big.NewInt(item.WeightedBytes))
	}
	for day := now.AddDate(0, -1, 0); !day.After(now); day = day.AddDate(0, 0, 1) {
		key := day.Format(time.DateOnly)
		result.Categories = append(result.Categories, key)
		value := "0"
		if daily[key] != nil {
			value = daily[key].String()
		}
		result.SparklineData = append(result.SparklineData, value)
	}
	nodes, err := s.Nodes(ctx)
	if err != nil {
		return result, Identity{}, err
	}
	for _, node := range nodes {
		if total := totals[node.ID]; total != nil && total.Sign() > 0 {
			multiplier := node.Multiplier
			result.TopNodes = append(result.TopNodes, model.TopNode{UUID: node.ID, Name: node.Name, CountryCode: node.CountryCode, TotalBytes: total.String(), ConsumptionMultiplier: &multiplier})
		}
	}
	return result, identity, nil
}
func (s *Service) Usage(ctx context.Context, id string, start, end time.Time) ([]DailyUsage, error) {
	identity, err := s.Identity(ctx, id)
	if err != nil {
		return nil, err
	}
	result, err := s.repo.XNodeUsage(ctx, identity.ProxyUserID, start, end)
	if err != nil {
		return nil, err
	}
	legacy, err := s.repo.XNodeLegacyHistory(ctx, identity.ProxyUserID, start, end)
	if err != nil {
		return nil, err
	}
	for _, value := range legacy {
		result = append(result, DailyUsage{ProxyUserID: value.ProxyUserID, NodeID: value.NodeID, Date: value.Date, UploadBytes: value.RawBytes, WeightedBytes: value.WeightedBytes})
	}
	return result, nil
}
