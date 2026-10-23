package xnode

import (
	"context"
	"math/big"
	"strconv"
	"time"
)

// ProfileUsage serializes exact daily counters as decimal strings.
type ProfileUsage struct {
	Date          string `json:"date"`
	UploadBytes   string `json:"uploadBytes"`
	DownloadBytes string `json:"downloadBytes"`
	WeightedBytes string `json:"weightedBytes"`
	LegacyBytes   string `json:"legacyBytes"`
}

type profileCounters struct{ upload, download, weighted, legacy big.Int }

func (c *profileCounters) add(row DailyUsage) error {
	if row.UploadBytes < 0 || row.DownloadBytes < 0 || row.WeightedBytes < 0 {
		return ErrInvalid
	}
	c.upload.Add(&c.upload, big.NewInt(row.UploadBytes))
	c.download.Add(&c.download, big.NewInt(row.DownloadBytes))
	c.weighted.Add(&c.weighted, big.NewInt(row.WeightedBytes))
	return nil
}

func (s *Service) profileAccounting(ctx context.Context, userID string, result ProfileMetrics) (ProfileMetrics, error) {
	identity, err := s.repo.XNodeIdentityByUser(ctx, userID)
	if err != nil {
		return result, err
	}
	statistics, _, err := s.accountStatistics(ctx, strconv.FormatInt(identity.ProxyUserID, 10))
	if err != nil {
		return result, err
	}
	result.UsedTrafficBytes, result.LifetimeTrafficBytes = statistics.UsedTrafficBytes, statistics.LifetimeTrafficBytes
	now := s.now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day()-6, 0, 0, 0, 0, time.UTC)
	usage, err := s.repo.XNodeUsage(ctx, identity.ProxyUserID, start, now)
	if err != nil {
		return result, err
	}
	daily, byNode := map[string]*profileCounters{}, map[string]*profileCounters{}
	for _, row := range usage {
		if daily[row.Date] == nil {
			daily[row.Date] = &profileCounters{}
		}
		if byNode[row.NodeID] == nil {
			byNode[row.NodeID] = &profileCounters{}
		}
		if err := daily[row.Date].add(row); err != nil {
			return result, err
		}
		if err := byNode[row.NodeID].add(row); err != nil {
			return result, err
		}
	}
	legacy, err := s.repo.XNodeImportedHistory(ctx, identity.ProxyUserID, start, now)
	if err != nil {
		return result, err
	}
	for _, row := range legacy {
		if row.RawBytes < 0 || row.WeightedBytes < 0 {
			return result, ErrInvalid
		}
		if daily[row.Date] == nil {
			daily[row.Date] = &profileCounters{}
		}
		if byNode[row.NodeID] == nil {
			byNode[row.NodeID] = &profileCounters{}
		}
		for _, values := range []*profileCounters{daily[row.Date], byNode[row.NodeID]} {
			values.legacy.Add(&values.legacy, big.NewInt(row.RawBytes))
			values.weighted.Add(&values.weighted, big.NewInt(row.WeightedBytes))
		}
	}
	for day := start; !day.After(now); day = day.AddDate(0, 0, 1) {
		key := day.Format(time.DateOnly)
		values := daily[key]
		if values == nil {
			values = &profileCounters{}
		}
		result.Usage = append(result.Usage, ProfileUsage{Date: key, UploadBytes: values.upload.String(), DownloadBytes: values.download.String(), WeightedBytes: values.weighted.String(), LegacyBytes: values.legacy.String()})
	}
	for i := range result.Nodes {
		if values := byNode[result.Nodes[i].NodeID]; values != nil {
			result.Nodes[i].UploadBytes, result.Nodes[i].DownloadBytes, result.Nodes[i].WeightedBytes = values.upload.String(), values.download.String(), values.weighted.String()
		}
	}
	return result, nil
}
