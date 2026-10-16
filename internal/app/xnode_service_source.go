package app

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnodemigration"
)

// The caller owns the existing Remnawave queue slot. Standalone accounts need
// the same complete daily history and exact aggregate capture as paid members.
func captureServiceAccount(ctx context.Context, client remnaClient, remote remnawave.User, name string) (xnodemigration.Account, error) {
	item := finalAccount(remote)
	item.ServiceName = name
	end := time.Now().UTC()
	stats, err := client.GetUserStats(ctx, remote.ID, remote.CreatedAt, end, rolloverStatsTopNodesLimit)
	if err != nil {
		return item, err
	}
	if _, err = validateRolloverStatistics(&remote, stats, remote.CreatedAt, end); err != nil {
		return item, err
	}
	item.Stats = *stats
	return item, nil
}
