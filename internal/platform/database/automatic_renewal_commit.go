package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (s *Store) CommitAutoRenewal(ctx context.Context, purchaseID string, now time.Time) (model.Purchase, error) {
	return s.retryAutoRenewal(ctx, purchaseID, nil, now)
}

// CommitAutoRenewalExcludingAddons excludes upstream-unavailable paid squads.
func (s *Store) CommitAutoRenewalExcludingAddons(ctx context.Context, purchaseID string, excludedAddonIDs []string, now time.Time) (model.Purchase, error) {
	return s.retryAutoRenewal(ctx, purchaseID, excludedAddonIDs, now)
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

func (s *Store) retryAutoRenewal(ctx context.Context, purchase string, excluded []string, now time.Time) (model.Purchase, error) {
	for attempt := 0; ; attempt++ {
		p, err := s.commitAutoRenewal(ctx, purchase, excluded, now)
		if !s.retryTransaction(ctx, err, attempt) {
			return p, err
		}
	}
}
