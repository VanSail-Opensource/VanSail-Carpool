package database

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// AutoRenewalFailure returns the latest member-visible automatic-renewal failure.
func (s *Store) AutoRenewalFailure(ctx context.Context, userID string) (*model.AutoRenewalFailure, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,auto_renew_failure_reason,auto_renew_failed_at,valid_until FROM purchases
		WHERE user_id=$1 AND auto_renew_failure_reason<>'' AND auto_renew_failed_at IS NOT NULL`, userID)
	if err != nil {
		return nil, fmt.Errorf("load automatic renewal failure: %w", err)
	}
	defer func() { _ = rows.Close() }()
	type candidate struct {
		failure model.AutoRenewalFailure
		until   time.Time
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var item candidate
		var rawFailed, rawUntil string
		if err := rows.Scan(&item.failure.PurchaseID, &item.failure.Reason, &rawFailed, &rawUntil); err != nil {
			return nil, err
		}
		item.failure.FailedAt, err = parseStamp(rawFailed)
		if err != nil {
			return nil, err
		}
		item.until, err = parseStamp(rawUntil)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, item)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, err
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].failure.FailedAt.Equal(candidates[j].failure.FailedAt) {
			return candidates[i].failure.PurchaseID < candidates[j].failure.PurchaseID
		}
		return candidates[i].failure.FailedAt.After(candidates[j].failure.FailedAt)
	})
	for _, item := range candidates {
		superseded, err := otherRenewalPurchaseEndsAfter(ctx, s.db, userID, item.failure.PurchaseID, item.until)
		if err != nil {
			return nil, err
		}
		if !superseded {
			return &item.failure, nil
		}
	}
	return nil, nil
}
