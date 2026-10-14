package catalog

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const dashboardNodeUsageMaximumDays = 31

var (
	// ErrDashboardNodeUsageRange marks an invalid member-selected usage range.
	ErrDashboardNodeUsageRange = errors.New("dashboard node usage range is invalid")
	// ErrDashboardNodeUsageUnavailable marks an unavailable live usage provider.
	ErrDashboardNodeUsageUnavailable = errors.New("dashboard node usage is unavailable")
)

type remoteUsageReader interface {
	NodeUsage(context.Context, string, time.Time, time.Time) (model.DashboardNodeUsage, error)
}

// NodeUsage returns a bounded, live per-node usage projection for the member.
// Remnawave treats start and end as date-only UTC values, inclusive.
func (s *Service) NodeUsage(ctx context.Context, user model.User, start, end time.Time) (model.DashboardNodeUsage, error) {
	if start.IsZero() || end.IsZero() || start.After(end) || end.Sub(start) > (dashboardNodeUsageMaximumDays-1)*24*time.Hour {
		return model.DashboardNodeUsage{}, ErrDashboardNodeUsageRange
	}
	if user.RemnaUserID == nil {
		return model.DashboardNodeUsage{}, ErrDashboardNodeUsageUnavailable
	}
	reader, ok := s.remnawave.(remoteUsageReader)
	if !ok {
		return model.DashboardNodeUsage{}, ErrDashboardNodeUsageUnavailable
	}
	report, err := reader.NodeUsage(ctx, *user.RemnaUserID, start.UTC(), end.UTC())
	if err != nil {
		return model.DashboardNodeUsage{}, fmt.Errorf("read dashboard node usage: %w", err)
	}
	report.StartDate = start.UTC().Format(time.DateOnly)
	report.EndDate = end.UTC().Format(time.DateOnly)
	return report, nil
}
