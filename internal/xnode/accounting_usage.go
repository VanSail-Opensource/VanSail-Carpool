package xnode

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usageaccounting"
)

// termPeriods selects the purchase, not the currently provisioned identity
// period. A queued settlement must never revoke a paid successor's streams.
func (s *Service) termPeriods(ctx context.Context, identity Identity, start, end time.Time, purchaseID ...string) ([]Period, error) {
	if end.Before(start) || identity.UserID == "" {
		return nil, ErrInvalid
	}
	purchases, err := s.repo.ListPurchases(ctx, identity.UserID)
	if err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	for _, purchase := range purchases {
		if (len(purchaseID) == 0 || purchase.ID == purchaseID[0]) && purchase.ValidFrom.Equal(start) && !purchase.ValidUntil.Before(end) {
			ids[purchase.ID] = true
		}
	}
	if len(ids) != 1 {
		return nil, ErrUnavailable
	}
	periods, err := s.repo.XNodePeriods(ctx, identity.ProxyUserID)
	if err != nil {
		return nil, err
	}
	result := []Period{}
	for _, period := range periods {
		if ids[period.PurchaseID] && !period.StartedAt.After(end) {
			result = append(result, period)
		}
	}
	if len(result) == 0 {
		return nil, ErrUnavailable
	}
	sort.Slice(result, func(i, j int) bool { return result[i].StartedAt.Before(result[j].StartedAt) })
	return result, nil
}

func (s *Service) CloseAccountingPeriod(ctx context.Context, id string, start, end time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, err := s.Identity(ctx, id)
	if err != nil {
		return err
	}
	periods, err := s.termPeriods(ctx, identity, start, end)
	if err != nil {
		return err
	}
	for _, period := range periods {
		if err = s.closePeriod(ctx, period); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) UsageSnapshot(ctx context.Context, id string, start, end time.Time) (usageaccounting.UsageSnapshot, error) {
	identity, err := s.Identity(ctx, id)
	if err != nil {
		return usageaccounting.UsageSnapshot{}, err
	}
	return s.accountingSnapshot(ctx, identity, start, end)
}

func (s *Service) accountingSnapshot(ctx context.Context, identity Identity, start, end time.Time, purchaseID ...string) (usageaccounting.UsageSnapshot, error) {
	result := usageaccounting.UsageSnapshot{Daily: []usageaccounting.DailyUsage{}}
	periods, err := s.termPeriods(ctx, identity, start, end, purchaseID...)
	if err != nil {
		return result, err
	}
	eligible := map[string]bool{}
	for _, period := range periods {
		if period.Status == "closing" || period.Status == "review" {
			return result, ErrUnavailable
		}
		eligible[period.ID] = true
		result.LimitBytes, result.Strategy, result.LastResetAt = period.LimitBytes, period.Strategy, period.LastResetAt
		used := period.UsedBytes
		result.CurrentUsedBytes = &used
	}
	usage, err := s.repo.XNodeUsage(ctx, identity.ProxyUserID, start, end)
	if err != nil {
		return result, err
	}
	daily := map[string]int64{}
	add := func(date string, bytes int64) error {
		at, err := time.Parse(time.DateOnly, date)
		if err != nil || bytes < 0 {
			return ErrInvalid
		}
		// The storage query includes the final UTC date. An exclusive midnight
		// boundary belongs to the next term and must be excluded here.
		if !at.Before(end) {
			return nil
		}
		if daily[date] > math.MaxInt64-bytes || result.WeightedUsedBytes > math.MaxInt64-bytes {
			return ErrUnavailable
		}
		daily[date] += bytes
		result.WeightedUsedBytes += bytes
		return nil
	}
	for _, sample := range usage {
		if eligible[sample.PeriodID] {
			if err = add(sample.Date, sample.WeightedBytes); err != nil {
				return result, err
			}
		}
	}
	legacy, err := s.repo.XNodeImportedHistory(ctx, identity.ProxyUserID, start, end)
	if err != nil {
		return result, err
	}
	for _, sample := range legacy {
		if err = add(sample.Date, sample.WeightedBytes); err != nil {
			return result, err
		}
	}
	for day, bytes := range daily {
		date, _ := time.Parse(time.DateOnly, day)
		result.Daily = append(result.Daily, usageaccounting.DailyUsage{Date: date, Bytes: bytes})
	}
	sort.Slice(result.Daily, func(i, j int) bool { return result.Daily[i].Date.Before(result.Daily[j].Date) })
	result.NodeSeriesAvailable = true
	result.TermUsageExact = true
	return result, nil
}
