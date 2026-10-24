package app

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
)

func (a *Application) observeDatabase(ctx context.Context) {
	if a.databaseOwner == nil {
		return
	}
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			pool := a.store.DB().Stats()
			tx := postgres.Snapshot(a.store.DB().Driver())
			traffic := a.xnodeMaster.TrafficMetrics()
			a.logger.Info("PostgreSQL runtime metrics", "pool_open", pool.OpenConnections, "pool_in_use", pool.InUse, "pool_max", pool.MaxOpenConnections, "pool_wait_total", pool.WaitCount, "pool_wait_seconds", pool.WaitDuration.Seconds(), "transactions_total", tx.Transactions, "transaction_errors_total", tx.Failures, "transaction_seconds_total", tx.Seconds, "serialization_retries_total", tx.Retries, "journal_commits_total", traffic.Commits, "journal_commit_seconds_total", traffic.CommitSeconds, "journal_acks_total", traffic.Acknowledged, "journal_ack_failures_total", traffic.AckFailures, "journal_commit_to_ack_seconds_total", traffic.AckSeconds)
		}
	}
}
