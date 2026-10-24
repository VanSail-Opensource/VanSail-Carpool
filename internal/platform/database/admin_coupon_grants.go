package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// GrantAdminCoupon records one idempotent, audited purchase-discount wallet grant.
func (s *Store) retryAttempt_GrantAdminCoupon(ctx context.Context, input AdminCouponGrantInput, now time.Time) (coupons.Grant, error) {
	if strings.TrimSpace(input.ActorUserID) == "" || strings.TrimSpace(input.UserID) == "" || strings.TrimSpace(input.CouponID) == "" || strings.TrimSpace(input.IdempotencyKey) == "" {
		return coupons.Grant{}, ErrConflict
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return coupons.Grant{}, err
	}
	defer func() { _ = tx.Rollback() }()
	sourceID := input.ActorUserID + ":" + input.IdempotencyKey
	var existingID, existingCouponID string
	err = tx.QueryRowContext(ctx, `SELECT id,coupon_id FROM coupon_grants WHERE user_id=? AND source_type='admin_grant' AND source_id=?`, input.UserID, sourceID).Scan(&existingID, &existingCouponID)
	if err == nil {
		if existingCouponID != input.CouponID {
			return coupons.Grant{}, ErrConflict
		}
		grant, loadErr := scanGrant(tx.QueryRowContext(ctx, grantSelect+` WHERE coupon_grants.id=?`, existingID))
		if loadErr != nil {
			return coupons.Grant{}, loadErr
		}
		return grant, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return coupons.Grant{}, err
	}
	coupon, err := couponByID(ctx, tx, input.CouponID)
	if err != nil {
		return coupons.Grant{}, err
	}
	if err = couponAvailable(coupon, now.UTC()); err != nil {
		return coupons.Grant{}, err
	}
	if err = ensureCouponUseAvailableTx(ctx, tx, coupon, input.UserID); err != nil {
		return coupons.Grant{}, err
	}
	grant, err := grantCouponTx(ctx, tx, input.UserID, coupon, "admin_grant", sourceID, now.UTC())
	if err != nil {
		return coupons.Grant{}, err
	}
	auditID, err := ids.New()
	if err != nil {
		return coupons.Grant{}, err
	}
	detail := fmt.Sprintf("coupon=%s reason=%s", coupon.ID, strings.TrimSpace(input.Reason))
	if err = insertAuditTx(ctx, tx, auditID, &input.ActorUserID, "coupon.grant", "coupon_grant", grant.ID, detail, now.UTC()); err != nil {
		return coupons.Grant{}, err
	}
	if err = tx.Commit(); err != nil {
		return coupons.Grant{}, err
	}
	return grant, nil
}

// DiscardAdminCoupon records administrative ownership before retaining the discard history.
func (s *Store) retryAttempt_DiscardAdminCoupon(ctx context.Context, actorID, userID, grantID, key string, now time.Time) error {
	actorID, userID, grantID, key = strings.TrimSpace(actorID), strings.TrimSpace(userID), strings.TrimSpace(grantID), strings.TrimSpace(key)
	if actorID == "" || userID == "" || grantID == "" || key == "" {
		return ErrConflict
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var replayUserID, replayGrantID string
	err = tx.QueryRowContext(ctx, `SELECT user_id,grant_id FROM admin_coupon_discard_commands WHERE actor_user_id=? AND idempotency_key=?`, actorID, key).Scan(&replayUserID, &replayGrantID)
	if err == nil {
		if replayUserID != userID || replayGrantID != grantID {
			return ErrConflict
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var status string
	err = tx.QueryRowContext(ctx, `SELECT status FROM coupon_grants WHERE id=? AND user_id=?`, grantID, userID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	var discarded int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM coupon_grant_discards WHERE grant_id=? AND user_id=?`, grantID, userID).Scan(&discarded)
	if errors.Is(err, sql.ErrNoRows) {
		if status != "active" {
			return ErrConflict
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO coupon_grant_discards(grant_id,user_id,discarded_at) VALUES(?,?,?)`, grantID, userID, stamp(now.UTC())); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO admin_coupon_discard_commands(actor_user_id,idempotency_key,user_id,grant_id,created_at) VALUES(?,?,?,?,?)`, actorID, key, userID, grantID, stamp(now.UTC())); err != nil {
		return err
	}
	auditID, err := ids.New()
	if err != nil {
		return err
	}
	if err = insertAuditTx(ctx, tx, auditID, &actorID, "coupon.discard", "coupon_grant", grantID, "admin profile discard", now.UTC()); err != nil {
		return err
	}
	return tx.Commit()
}
