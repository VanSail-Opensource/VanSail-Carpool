package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type usageBreakdownScope struct {
	owner               string
	generation          int64
	lifecycle, from, to time.Time
	rawFrom, paygFrom   time.Time
}

// XNodeUsageBreakdown reads one owner-consistent snapshot without touching
// counters, purchases, wallet state, upstream services or the history floor.
func (s *Store) XNodeUsageBreakdown(ctx context.Context, owner string, from, to time.Time) (model.UsageBreakdown, error) {
	result := emptyUsageBreakdown(from.UTC(), to.UTC())
	if owner == "" || len(owner) > 128 || from.IsZero() || to.IsZero() || to.Before(from) || to.Sub(from) > xnode.PAYGWindow+time.Minute {
		return result, xnode.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	scope, err := usageBreakdownOwnerTx(ctx, tx, owner, from.UTC(), to.UTC())
	if err != nil {
		return result, err
	}
	legacy, err := usageBreakdownLegacyTx(ctx, tx, scope)
	if err != nil {
		return result, err
	}
	unallocated, err := usageBreakdownUnallocatedTx(ctx, tx, scope)
	if err != nil {
		return result, err
	}
	result.Coverage, err = usageBreakdownCoverageTx(ctx, tx, scope)
	if err != nil {
		return result, err
	}
	if err = readUsageBreakdownNodesTx(ctx, tx, scope, legacy, unallocated, &result); err != nil {
		return result, err
	}
	if err = readUsageBreakdownHourlyTx(ctx, tx, scope, &result); err != nil {
		return result, err
	}
	result.Coverage.Complete = result.Coverage.Since != nil && !result.Coverage.Since.After(from) && result.Coverage.GapCount == "0" && !result.Coverage.Truncated
	if err = tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}

func usageBreakdownOwnerTx(ctx context.Context, tx *sql.Tx, owner string, from, to time.Time) (usageBreakdownScope, error) {
	scope := usageBreakdownScope{owner: owner, from: from, to: to}
	var state, created string
	err := tx.QueryRowContext(ctx, `SELECT account_generation,account_status,created_at FROM users WHERE id=$1`, owner).Scan(&scope.generation, &state, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return scope, ErrNotFound
	}
	if err != nil {
		return scope, err
	}
	if state != "active" {
		return scope, ErrAccountUnavailable
	}
	scope.lifecycle, err = parseStamp(created)
	if err != nil {
		return scope, err
	}
	if scope.lifecycle.After(scope.from) {
		scope.from = scope.lifecycle
	}
	scope.rawFrom = scope.from.Truncate(time.Minute)
	scope.paygFrom = scope.rawFrom
	// PAYG buckets have no original observation bounds or generation. Never
	// assign an unsplittable previous-lifecycle bucket to a newly joined owner.
	if scope.lifecycle.After(scope.paygFrom) {
		scope.paygFrom = scope.lifecycle.Truncate(time.Minute).Add(time.Minute)
	}
	return scope, nil
}

func emptyUsageBreakdown(from, to time.Time) model.UsageBreakdown {
	zero := model.RationalMinor{Numerator: "0", Denominator: "1"}
	return model.UsageBreakdown{
		From: from, To: to, Coverage: model.UsageCoverage{GapCount: "0"},
		Nodes: []model.UsageBreakdownNode{}, HourlySquads: []model.UsageBreakdownHourlySquad{},
		Omitted:         model.UsageBreakdownTotals{UploadBytes: "0", DownloadBytes: "0", CostMinor: zero},
		PositiveNodes:   model.UsageBreakdownPositiveNodes{Traffic: "0", Cost: "0"},
		PositiveEntries: model.UsageBreakdownPositiveNodes{Traffic: "0", Cost: "0"},
		Cost: model.UsageBreakdownCost{
			ActualMinor: "0", Legacy: model.UsageLegacyEstimate{Status: model.UsageLegacyNone},
			KnownTotalMinor: zero, UnallocatedMinor: zero,
		},
	}
}
