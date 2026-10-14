package database

import (
	"context"
	"database/sql"
	"fmt"
	"math"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

type drawPriceContext struct {
	avgBalance                                *float64
	minHourly, maxHourly                      float64
	cheapestCombo, highestCombo, highestQuote int64
}

func (s *Store) drawPriceContext(ctx context.Context) (drawPriceContext, error) {
	var value drawPriceContext
	var average sql.NullFloat64
	if err := s.db.QueryRowContext(ctx, `SELECT AVG(b.txb_minor) FROM balances b
		JOIN users u ON u.id=b.user_id WHERE u.role='user' AND b.txb_minor>=100`).Scan(&average); err != nil {
		return value, err
	}
	if average.Valid {
		value.avgBalance = &average.Float64
	}
	rows, err := s.db.QueryContext(ctx, `SELECT price_txb_minor,validity_days FROM combos WHERE active=1 AND validity_days>0`)
	if err != nil {
		return value, err
	}
	defer func() { _ = rows.Close() }()
	value.minHourly = math.Inf(1)
	seenCombo := false
	for rows.Next() {
		var price int64
		var days int
		if err = rows.Scan(&price, &days); err != nil {
			return value, err
		}
		hourly := float64(price) / float64(days*24)
		value.minHourly = math.Min(value.minHourly, hourly)
		value.maxHourly = math.Max(value.maxHourly, hourly)
		if !seenCombo || price < value.cheapestCombo {
			value.cheapestCombo = price
		}
		seenCombo = true
		if price > value.highestCombo {
			value.highestCombo = price
		}
	}
	if err = rows.Err(); err != nil {
		return value, err
	}
	if math.IsInf(value.minHourly, 1) {
		value.minHourly = 0
	}
	value.highestQuote = value.highestCombo
	squadRows, err := s.db.QueryContext(ctx, `SELECT price_txb_minor FROM squad_product_overrides WHERE visible=1 ORDER BY price_txb_minor DESC LIMIT 100`)
	if err != nil {
		return value, err
	}
	defer func() { _ = squadRows.Close() }()
	for squadRows.Next() {
		var price int64
		if err = squadRows.Scan(&price); err != nil {
			return value, err
		}
		if value.highestQuote > math.MaxInt64-price {
			return value, ErrConflict
		}
		value.highestQuote += price
	}
	if err = squadRows.Err(); err != nil {
		return value, err
	}
	return value, nil
}
func (s *Store) drawPrizeExpense(ctx context.Context, price drawPriceContext, reward activity.Reward) (float64, float64, bool, error) {
	minimum, maximum := float64(0), float64(0)
	oneTerm := false
	low, high := float64(0), float64(0)
	if reward.Range != nil {
		low, high = float64(reward.Range.Min), float64(reward.Range.Max)
	}
	switch reward.Kind {
	case activity.RewardNone, activity.RewardTrafficGrant, activity.RewardTrafficReset:
	case activity.RewardTXBDelta:
		if reward.Range != nil {
			minimum, maximum = low, high
		} else {
			minimum, maximum = float64(reward.TXBDeltaMinor), float64(reward.TXBDeltaMinor)
		}
	case activity.RewardBalanceMultiplier:
		if price.avgBalance == nil {
			return 0, 0, false, ErrConflict
		}
		minimum = (low/10000 - 1) * (*price.avgBalance)
		maximum = (high/10000 - 1) * (*price.avgBalance)
	case activity.RewardSubscriptionExtension:
		if reward.Range == nil {
			low, high = float64(reward.ExtensionDays*24), float64(reward.ExtensionDays*24)
		}
		minimum, maximum = low*price.minHourly, high*price.maxHourly
	case activity.RewardEntitlementGrant:
		minimum, maximum = float64(reward.RenewalPriceMinor), float64(reward.RenewalPriceMinor)
	case activity.RewardSquadAccess:
		for _, uuid := range reward.SquadUUIDs {
			var squadPrice sql.NullInt64
			err := s.db.QueryRowContext(ctx, `SELECT price_txb_minor FROM squad_product_overrides WHERE remna_squad_uuid=?`, uuid).Scan(&squadPrice)
			if err != nil && err != sql.ErrNoRows {
				return 0, 0, false, err
			}
			if squadPrice.Valid {
				minimum += float64(squadPrice.Int64)
				maximum += float64(squadPrice.Int64)
			}
		}
	case activity.RewardCoreComboSwitch:
		var selected int64
		if err := s.db.QueryRowContext(ctx, `SELECT price_txb_minor FROM combos WHERE id=?`, reward.ComboID).Scan(&selected); err != nil {
			return 0, 0, false, err
		}
		maximum = float64(max(0, selected-price.cheapestCombo))
	case activity.RewardCouponRecurring, activity.RewardCouponOnce:
		oneTerm = true
		if reward.DiscountMode == "fixed" {
			maximum = math.Min(high, float64(price.highestQuote))
		} else {
			maximum = float64(price.highestQuote) * high / 10000
		}
	default:
		return 0, 0, false, fmt.Errorf("unknown reward kind %q", reward.Kind)
	}
	return minimum, maximum, oneTerm, nil
}
