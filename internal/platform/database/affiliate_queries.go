package database

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/affiliates"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (s *Store) AffiliateOverview(ctx context.Context, userID, botUsername string) (affiliates.Overview, error) {
	var result affiliates.Overview
	var telegramID int64
	var totalCommission int64
	err := s.db.QueryRowContext(ctx, `SELECT u.telegram_id,COUNT(i.id),COUNT(a.id),COALESCE(SUM(a.commission_txb_minor),0)
		FROM users u LEFT JOIN users i ON i.inviter_id=u.telegram_id LEFT JOIN affiliate_settlements a ON a.invited_user_id=i.id
		WHERE u.id=? GROUP BY u.id`, userID).Scan(&telegramID, &result.RegisteredCount, &result.SuccessfulCount, &totalCommission)
	if err != nil {
		return result, err
	}
	result.TotalCommission.Currency = "TXB"
	result.TotalCommission.Minor = strconv.FormatInt(totalCommission, 10)
	result.TotalCommission.Display = model.TXBMoney(totalCommission).Display
	if botUsername != "" {
		link := fmt.Sprintf("https://t.me/%s?start=%d", botUsername, telegramID)
		result.InviteLink = &link
	}
	if result.RegisteredCount > 0 {
		result.ConversionBPS = result.SuccessfulCount * 10000 / result.RegisteredCount
	}
	config, err := s.AffiliateConfig(ctx)
	if err != nil {
		return result, err
	}
	result.TierProgress = progressFor(config.Tiers, result.SuccessfulCount)
	return result, nil
}

func progressFor(tiers []affiliates.Tier, successful int) affiliates.TierProgress {
	enabled := make([]affiliates.Tier, 0, len(tiers))
	for _, tier := range tiers {
		if tier.Enabled {
			enabled = append(enabled, tier)
		}
	}
	current := enabled[0]
	for _, tier := range enabled {
		if tier.Threshold <= successful {
			current = tier
		}
	}
	progress := affiliates.TierProgress{Current: current, Successful: successful, TopTier: true}
	for i := range enabled {
		if enabled[i].Threshold > successful {
			progress.Next = &enabled[i]
			progress.Remaining = enabled[i].Threshold - successful
			progress.TopTier = false
			break
		}
	}
	return progress
}

func (s *Store) AffiliateReferrals(ctx context.Context, userID string, page int) (affiliates.ReferralPage, error) {
	if page < 1 {
		page = 1
	}
	result := affiliates.ReferralPage{Page: page, PageSize: affiliates.PageSize, Items: []affiliates.Referral{}}
	var telegramID int64
	if err := s.db.QueryRowContext(ctx, `SELECT telegram_id FROM users WHERE id=?`, userID).Scan(&telegramID); err != nil {
		return result, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE inviter_id=?`, telegramID).Scan(&result.Total); err != nil {
		return result, err
	}
	result.TotalPages = (result.Total + affiliates.PageSize - 1) / affiliates.PageSize
	rows, err := s.db.QueryContext(ctx, `SELECT COALESCE(i.telegram_first_name,''),COALESCE(i.telegram_last_name,''),i.created_at,
		a.settled_at,a.commission_txb_minor FROM users i LEFT JOIN affiliate_settlements a ON a.invited_user_id=i.id
		WHERE i.inviter_id=? ORDER BY i.created_at DESC,i.id DESC LIMIT ? OFFSET ?`, telegramID, affiliates.PageSize, (page-1)*affiliates.PageSize)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var item affiliates.Referral
		var registered string
		var paidAt sql.NullString
		var amount sql.NullInt64
		if err := rows.Scan(&item.FirstName, &item.LastName, &registered, &paidAt, &amount); err != nil {
			return result, err
		}
		item.RegisteredAt, err = parseStamp(registered)
		if err != nil {
			return result, err
		}
		item.Status = "pending"
		if paidAt.Valid {
			paid, parseErr := parseStamp(paidAt.String)
			if parseErr != nil {
				return result, parseErr
			}
			item.Status, item.PaybackAt = "successful", &paid
			money := affiliates.Money{Minor: strconv.FormatInt(amount.Int64, 10), Currency: "TXB", Display: model.TXBMoney(amount.Int64).Display}
			item.CommissionAmount = &money
		}
		result.Items = append(result.Items, item)
	}
	return result, rows.Err()
}

// SuccessfulAffiliateReferrals returns only settled referrals for an administrator profile.
func (s *Store) SuccessfulAffiliateReferrals(ctx context.Context, userID string, limit int) (affiliates.ReferralPage, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	result := affiliates.ReferralPage{Page: 1, PageSize: limit, Items: []affiliates.Referral{}}
	var telegramID int64
	if err := s.db.QueryRowContext(ctx, `SELECT telegram_id FROM users WHERE id=?`, userID).Scan(&telegramID); err != nil {
		return result, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users i JOIN affiliate_settlements a ON a.invited_user_id=i.id WHERE i.inviter_id=?`, telegramID).Scan(&result.Total); err != nil {
		return result, err
	}
	result.TotalPages = (result.Total + limit - 1) / limit
	rows, err := s.db.QueryContext(ctx, `SELECT COALESCE(i.telegram_first_name,''),COALESCE(i.telegram_last_name,''),i.created_at,a.settled_at,a.commission_txb_minor
		FROM users i JOIN affiliate_settlements a ON a.invited_user_id=i.id WHERE i.inviter_id=? ORDER BY a.settled_at DESC,i.id DESC LIMIT ?`, telegramID, limit)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var item affiliates.Referral
		var registered, paidAt string
		var amount int64
		if err := rows.Scan(&item.FirstName, &item.LastName, &registered, &paidAt, &amount); err != nil {
			return result, err
		}
		var err error
		if item.RegisteredAt, err = parseStamp(registered); err != nil {
			return result, err
		}
		paid, err := parseStamp(paidAt)
		if err != nil {
			return result, err
		}
		item.Status, item.PaybackAt = "successful", &paid
		money := affiliates.Money{Minor: strconv.FormatInt(amount, 10), Currency: "TXB", Display: model.TXBMoney(amount).Display}
		item.CommissionAmount = &money
		result.Items = append(result.Items, item)
	}
	return result, rows.Err()
}
