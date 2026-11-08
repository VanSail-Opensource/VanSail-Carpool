package xnode

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// These forward observations are separate from wallet charges and immutable
// purchase terms. The caller must not infer full history before Coverage.Since.
type UsageHistoryRow struct {
	BillingMode, ReferenceID, NodeID, Name, CountryCode string
	UploadBytes, DownloadBytes                          string
	Hour                                                time.Time
}

type UsageCoverage = model.UsageCoverage

type UsageHistory struct {
	From, To time.Time
	Rows     []UsageHistoryRow
	Coverage UsageCoverage
}

type usageHistoryRepository interface {
	XNodeUsageHistory(context.Context, string, time.Time, time.Time) ([]UsageHistoryRow, UsageCoverage, error)
}

func (s *Service) OwnedUsageHistory(ctx context.Context, owner string) (UsageHistory, error) {
	now := s.now().UTC()
	result := UsageHistory{From: now.Add(-PAYGWindow).Truncate(time.Minute), To: now, Rows: []UsageHistoryRow{}}
	if repo, ok := s.repo.(usageHistoryRepository); ok {
		var err error
		result.Rows, result.Coverage, err = repo.XNodeUsageHistory(ctx, owner, result.From, result.To)
		return result, err
	}
	return result, nil
}
