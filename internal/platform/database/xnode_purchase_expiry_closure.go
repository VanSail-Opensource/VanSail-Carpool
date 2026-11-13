package database

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// The due decision and period admission share the purchase lock. An expiry job
// captured before an extension cannot close that newer paid allowance.
func (s *Store) retryAttempt_BeginXNodePurchaseExpiryClosure(ctx context.Context, user model.User, purchase model.Purchase, expected xnode.Period, at time.Time) (xnode.Period, bool, error) {
	if user.ID == "" || purchase.ID == "" || purchase.UserID != user.ID || expected.PurchaseID != purchase.ID || at.IsZero() {
		return xnode.Period{}, false, xnode.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return xnode.Period{}, false, err
	}
	defer tx.Rollback()
	if err = lockNativeMemberIdentityTx(ctx, tx, user.ID, user.AccountGeneration, true); err != nil {
		return xnode.Period{}, false, err
	}
	var status, rawUntil, rawUpdated string
	err = tx.QueryRowContext(ctx, `SELECT status,valid_until,updated_at FROM purchases WHERE id=$1 AND user_id=$2 FOR UPDATE`, purchase.ID, user.ID).Scan(&status, &rawUntil, &rawUpdated)
	if errors.Is(err, sql.ErrNoRows) {
		return xnode.Period{}, false, nil
	}
	if err != nil {
		return xnode.Period{}, false, err
	}
	until, err := parseStamp(rawUntil)
	if err != nil {
		return xnode.Period{}, false, err
	}
	updated, err := parseStamp(rawUpdated)
	if err != nil {
		return xnode.Period{}, false, err
	}
	if status == "cancelled" || until.After(at) || !updated.Equal(purchase.UpdatedAt) {
		return xnode.Period{}, false, nil
	}
	var owner string
	if err = tx.QueryRowContext(ctx, `SELECT user_id FROM xnode_identities WHERE proxy_user_id=$1 FOR SHARE`, expected.ProxyUserID).Scan(&owner); err != nil {
		return xnode.Period{}, false, err
	}
	if owner != user.ID {
		return xnode.Period{}, false, xnode.ErrConflict
	}
	args := []any{expected.ID, expected.ProxyUserID, purchase.ID, expected.ClosureGeneration}
	query := `SELECT ` + xnodePeriodColumns + ` FROM xnode_periods WHERE id=$1 AND proxy_user_id=$2 AND purchase_id=$3 AND closure_generation=$4 AND status=$5 FOR UPDATE`
	if expected.Status == "open" {
		if expected.ClosureGeneration == math.MaxInt64 {
			return xnode.Period{}, false, xnode.ErrConflict
		}
		query = `UPDATE xnode_periods SET status='closing',ended_at=$5,closure_generation=closure_generation+1 WHERE id=$1 AND proxy_user_id=$2 AND purchase_id=$3 AND closure_generation=$4 AND status='open' RETURNING ` + xnodePeriodColumns
		args = append(args, stamp(at))
	} else if expected.Status == "closing" || expected.Status == "closed" {
		args = append(args, expected.Status)
	} else {
		return xnode.Period{}, false, xnode.ErrConflict
	}
	period, err := scanXNodePeriod(tx.QueryRowContext(ctx, query, args...))
	if errors.Is(err, xnode.ErrNotFound) {
		err = xnode.ErrConflict
	}
	if err != nil {
		return xnode.Period{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return xnode.Period{}, false, err
	}
	return period, true, nil
}
