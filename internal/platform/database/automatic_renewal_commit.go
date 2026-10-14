package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// CommitAutoRenewal atomically settles one calculated rollover and successor.
func (s *Store) CommitAutoRenewal(ctx context.Context, purchaseID string, now time.Time) (model.Purchase, error) {
	return s.commitAutoRenewal(ctx, purchaseID, nil, true, now)
}

// CommitAutoRenewalExcludingAddons excludes upstream-unavailable paid squads.
func (s *Store) CommitAutoRenewalExcludingAddons(ctx context.Context, purchaseID string, excludedAddonIDs []string, now time.Time) (model.Purchase, error) {
	return s.commitAutoRenewal(ctx, purchaseID, excludedAddonIDs, true, now)
}

// CommitAutoRenewalWithRolloverBalance settles automatic renewal under the
// current global policy for whether rollover credit counts toward its balance.
func (s *Store) CommitAutoRenewalWithRolloverBalance(ctx context.Context, purchaseID string, rolloverCountsTowardBalance bool, now time.Time) (model.Purchase, error) {
	return s.commitAutoRenewal(ctx, purchaseID, nil, rolloverCountsTowardBalance, now)
}

// CommitAutoRenewalExcludingAddonsWithRolloverBalance also excludes live-unavailable paid squads.
func (s *Store) CommitAutoRenewalExcludingAddonsWithRolloverBalance(ctx context.Context, purchaseID string, excludedAddonIDs []string, rolloverCountsTowardBalance bool, now time.Time) (model.Purchase, error) {
	return s.commitAutoRenewal(ctx, purchaseID, excludedAddonIDs, rolloverCountsTowardBalance, now)
}

func automaticRenewalSuccessorIDTx(ctx context.Context, tx *sql.Tx, sourceID string) (string, bool, error) {
	var successorID string
	err := tx.QueryRowContext(ctx, `SELECT id FROM purchases WHERE auto_renew_source_purchase_id=?`, sourceID).Scan(&successorID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("load automatic renewal successor: %w", err)
	}
	return successorID, true, nil
}

func rolloverStatusForCredit(credit int64) string {
	if credit > 0 {
		return "credited"
	}
	return "zero"
}
