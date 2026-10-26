package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/X-Node/pkg/compute"
)

func couponFingerprint(grants []coupons.Grant) string {
	if grants == nil {
		grants = []coupons.Grant{}
	}
	raw, _ := json.Marshal(grants)
	return compute.Digest(raw)
}

func (s *Store) retryAttempt_offerCouponInputs(ctx context.Context, user, kind, product string, now time.Time) ([]coupons.Grant, error) {
	if kind == "payg" {
		return []coupons.Grant{}, nil
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	combo, squads := product, []string{}
	if kind == "combo" {
		c, e := comboByIDTx(ctx, tx, product, false)
		if e != nil {
			return nil, e
		}
		for _, g := range c.IncludedSquads {
			squads = append(squads, g.GroupID)
		}
	} else {
		squads = []string{product}
		err = tx.QueryRowContext(ctx, `SELECT combo_id FROM purchases WHERE user_id=? AND status IN ('active','activating') AND valid_until>? ORDER BY valid_until DESC LIMIT 1`, user, pricingStamp(now)).Scan(&combo)
		if errors.Is(err, sql.ErrNoRows) {
			combo = ""
		} else if err != nil {
			return nil, err
		}
	}
	return pricingCouponsTx(ctx, tx, user, combo, squads, now)
}
