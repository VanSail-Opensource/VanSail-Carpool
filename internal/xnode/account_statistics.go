package xnode

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
	"math/big"
	"strconv"
	"time"
)

func (s *Service) MemberOperationState(ctx context.Context, id string) (purchaseops.RemoteState, error) {
	identity, err := s.Identity(ctx, id)
	if err != nil {
		return purchaseops.RemoteState{}, err
	}
	if identity.CurrentPeriodID == "" {
		return purchaseops.RemoteState{Quiesced: true}, nil
	}
	period, err := s.repo.XNodePeriod(ctx, identity.CurrentPeriodID)
	if err != nil {
		return purchaseops.RemoteState{}, err
	}
	// "closing" is not settled: never allow a refund to use an unfinished count.
	if period.Status == "closing" {
		if err = s.Quiesce(ctx, id); err != nil {
			return purchaseops.RemoteState{}, err
		}
		period, err = s.repo.XNodePeriod(ctx, period.ID)
		if err == nil && period.Status != "closed" {
			return purchaseops.RemoteState{}, ErrUnavailable
		}
	}
	return purchaseops.RemoteState{UsedTrafficBytes: period.UsedBytes, LastTrafficResetAt: period.LastResetAt, Quiesced: period.Status == "closed"}, err
}
func (s *Service) Statistics(ctx context.Context, id string) (model.Statistics, string, error) {
	identity, err := s.Identity(ctx, id)
	if err != nil {
		return model.Statistics{}, "", err
	}
	result := model.Statistics{UsedTrafficBytes: "0", LifetimeTrafficBytes: "0", TrafficLimitBytes: "0", Categories: []string{}, SparklineData: []string{}, TopNodes: []model.TopNode{}}
	if identity.CurrentPeriodID != "" {
		period, err := s.repo.XNodePeriod(ctx, identity.CurrentPeriodID)
		if err != nil {
			return result, "", err
		}
		result.UsedTrafficBytes = strconv.FormatInt(period.UsedBytes, 10)
		result.TrafficLimitBytes = strconv.FormatInt(period.LimitBytes, 10)
		result.LastTrafficResetAt = period.LastResetAt
	}
	periods, err := s.repo.XNodePeriods(ctx, identity.ProxyUserID)
	if err != nil {
		return result, "", err
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
			return result, "", err
		}
		for _, sample := range all {
			lifetime.Add(lifetime, big.NewInt(sample.WeightedBytes))
		}
	} else if !isMissing(legacyErr) {
		return result, "", legacyErr
	}
	result.LifetimeTrafficBytes = lifetime.String()
	now := s.now().UTC()
	usage, err := s.repo.XNodeUsage(ctx, identity.ProxyUserID, now.AddDate(0, -1, 0), now)
	if err != nil {
		return result, "", err
	}
	legacyHistory, err := s.repo.XNodeLegacyHistory(ctx, identity.ProxyUserID, now.AddDate(0, -1, 0), now)
	if err != nil {
		return result, "", err
	}
	for _, value := range legacyHistory {
		usage = append(usage, DailyUsage{ProxyUserID: value.ProxyUserID, NodeID: value.NodeID, Date: value.Date, UploadBytes: value.RawBytes, WeightedBytes: value.WeightedBytes})
	}
	daily := map[string]int64{}
	totals := map[string]int64{}
	for _, item := range usage {
		daily[item.Date] += item.WeightedBytes
		totals[item.NodeID] += item.WeightedBytes
	}
	for day := now.AddDate(0, -1, 0); !day.After(now); day = day.AddDate(0, 0, 1) {
		key := day.Format(time.DateOnly)
		result.Categories = append(result.Categories, key)
		result.SparklineData = append(result.SparklineData, strconv.FormatInt(daily[key], 10))
	}
	nodes, err := s.Nodes(ctx)
	if err != nil {
		return result, "", err
	}
	for _, node := range nodes {
		if totals[node.ID] > 0 {
			multiplier := node.Multiplier
			result.TopNodes = append(result.TopNodes, model.TopNode{UUID: node.ID, Name: node.Name, CountryCode: node.CountryCode, TotalBytes: strconv.FormatInt(totals[node.ID], 10), ConsumptionMultiplier: &multiplier})
		}
	}
	link, err := s.SubscriptionURL(identity)
	return result, link, err
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
