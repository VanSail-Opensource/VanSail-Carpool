package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/affiliates"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

func (s *Store) AffiliateConfig(ctx context.Context) (affiliates.Config, error) {
	return affiliateConfig(ctx, s.db)
}

func affiliateConfig(ctx context.Context, query interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}) (affiliates.Config, error) {
	var config affiliates.Config
	err := query.QueryRowContext(ctx, `SELECT c.id,c.version FROM affiliate_config_versions c
		JOIN affiliate_config_current current ON current.config_id=c.id WHERE current.singleton=1`).Scan(&config.ID, &config.Version)
	if err != nil {
		return config, err
	}
	rows, err := query.QueryContext(ctx, `SELECT t.id,t.name,t.success_threshold,t.enabled,t.commission_enabled,t.commission_bps,
		t.reward_kind,COALESCE(t.reward_coupon_id,''),COALESCE(c.name,''),COALESCE(t.reward_txb_minor,0),COALESCE(t.reward_extension_days,0)
		FROM affiliate_tiers t LEFT JOIN coupon_definitions c ON c.id=t.reward_coupon_id WHERE t.config_id=? ORDER BY t.position`, config.ID)
	if err != nil {
		return config, err
	}
	defer rows.Close()
	for rows.Next() {
		var tier affiliates.Tier
		var enabled, commission int
		if err := rows.Scan(&tier.ID, &tier.Name, &tier.Threshold, &enabled, &commission, &tier.CommissionBPS,
			&tier.Reward.Kind, &tier.Reward.CouponID, &tier.Reward.CouponName, &tier.Reward.TXBMinor, &tier.Reward.ExtensionDays); err != nil {
			return config, err
		}
		tier.Enabled, tier.CommissionEnabled = enabled == 1, commission == 1
		config.Tiers = append(config.Tiers, tier)
	}
	return config, rows.Err()
}

func (s *Store) SaveAffiliateConfig(ctx context.Context, actorID string, input affiliates.ConfigInput, now time.Time) (affiliates.Config, error) {
	if input.ExpectedVersion < 1 || affiliates.ValidateTiers(input.Tiers) != nil {
		return affiliates.Config{}, affiliates.ErrInvalidInput
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return affiliates.Config{}, err
	}
	defer func() { _ = tx.Rollback() }()
	current, err := affiliateConfig(ctx, tx)
	if err != nil {
		return affiliates.Config{}, err
	}
	if current.Version != input.ExpectedVersion {
		return affiliates.Config{}, affiliates.ErrVersionConflict
	}
	if err := validateAffiliateCoupons(ctx, tx, input.Tiers, now); err != nil {
		return affiliates.Config{}, err
	}
	configID, err := ids.New()
	if err != nil {
		return affiliates.Config{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO affiliate_config_versions(id,version,created_by,created_at) VALUES(?,?,?,?)`, configID, current.Version+1, actorID, stamp(now)); err != nil {
		return affiliates.Config{}, err
	}
	for position, tier := range input.Tiers {
		tierID, idErr := ids.New()
		if idErr != nil {
			return affiliates.Config{}, idErr
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO affiliate_tiers(id,config_id,position,name,success_threshold,enabled,commission_enabled,commission_bps,
			reward_kind,reward_coupon_id,reward_txb_minor,reward_extension_days) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
			tierID, configID, position, tier.Name, tier.Threshold, boolInt(tier.Enabled), boolInt(tier.CommissionEnabled), tier.CommissionBPS,
			tier.Reward.Kind, nullableText(tier.Reward.CouponID), nullablePositive(tier.Reward.TXBMinor), nullablePositiveInt(tier.Reward.ExtensionDays))
		if err != nil {
			return affiliates.Config{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE affiliate_config_current SET config_id=? WHERE singleton=1`, configID); err != nil {
		return affiliates.Config{}, err
	}
	detail, _ := json.Marshal(map[string]any{"previousVersion": current.Version, "version": current.Version + 1})
	auditID, err := ids.New()
	if err != nil {
		return affiliates.Config{}, err
	}
	if err := insertAuditTx(ctx, tx, auditID, &actorID, "affiliate.config.update", "affiliate_config", configID, string(detail), now); err != nil {
		return affiliates.Config{}, err
	}
	if err := tx.Commit(); err != nil {
		return affiliates.Config{}, err
	}
	return s.AffiliateConfig(ctx)
}

func validateAffiliateCoupons(ctx context.Context, tx *sql.Tx, tiers []affiliates.Tier, now time.Time) error {
	for _, tier := range tiers {
		if tier.Reward.Kind != "coupon" {
			continue
		}
		coupon, err := couponByID(ctx, tx, tier.Reward.CouponID)
		if errors.Is(err, ErrNotFound) {
			return affiliates.ErrInvalidInput
		}
		if err != nil {
			return err
		}
		if coupon.Kind != coupons.KindPurchaseOnce && coupon.Kind != coupons.KindPurchaseRecurring {
			return affiliates.ErrInvalidInput
		}
		if err := couponAvailable(coupon, now); err != nil {
			return affiliates.ErrInvalidInput
		}
	}
	return nil
}

func nullableText(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func nullablePositive(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}
func nullablePositiveInt(value int) any {
	if value <= 0 {
		return nil
	}
	return value
}
