package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/affiliates"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

func awardCrossedAffiliateTierTx(ctx context.Context, tx *sql.Tx, config affiliates.Config, inviterID string, telegramID int64,
	settlementID string, successful int, locale string, now time.Time) error {
	for _, tier := range config.Tiers {
		if !tier.Enabled || tier.Threshold == 0 || tier.Threshold != successful {
			continue
		}
		awardID, err := ids.New()
		if err != nil {
			return err
		}
		description := affiliateRewardDescription(tier.Reward, locale)
		_, err = tx.ExecContext(ctx, `INSERT INTO affiliate_tier_awards(id,inviter_user_id,tier_id,settlement_id,tier_name,reward_kind,reward_description,
			reward_coupon_id,reward_txb_minor,reward_extension_days,awarded_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, awardID, inviterID, tier.ID, settlementID,
			tier.Name, tier.Reward.Kind, description, nullableText(tier.Reward.CouponID), nullablePositive(tier.Reward.TXBMinor), nullablePositiveInt(tier.Reward.ExtensionDays), stamp(now))
		if err != nil {
			return err
		}
		if tier.Reward.Kind != "none" {
			if err := applyAffiliateRewardTx(ctx, tx, inviterID, awardID, tier.Reward, now); err != nil {
				return err
			}
		}
		payload := jobpayload.AffiliateTierUpgrade{AwardID: awardID, ChatID: telegramID, Locale: locale, TierName: tier.Name,
			RewardDescription: description, AwardedAt: stamp(now)}
		return insertAffiliatePayloadTx(ctx, tx, jobpayload.AffiliateTierUpgradeKind, payload, now)
	}
	return nil
}

func applyAffiliateRewardTx(ctx context.Context, tx *sql.Tx, userID, awardID string, reward affiliates.Reward, now time.Time) error {
	switch reward.Kind {
	case "none":
		return nil
	case "coupon":
		coupon, err := couponByID(ctx, tx, reward.CouponID)
		if err != nil {
			return err
		}
		_, err = grantCouponTx(ctx, tx, userID, coupon, "affiliate_tier", awardID, now)
		return err
	case "txb":
		balance, err := adjustBalanceTx(ctx, tx, userID, reward.TXBMinor, now)
		if err != nil {
			return err
		}
		_, err = insertLedgerTx(ctx, tx, userID, reward.TXBMinor, balance, "affiliate_tier_reward", awardID, "affiliate tier upgrade", now)
		return err
	case "subscription_extension":
		return applySubscriptionExtensionTx(ctx, tx, userID, reward.ExtensionDays, "affiliate_tier", awardID, now)
	default:
		return affiliates.ErrInvalidInput
	}
}

func affiliateRewardDescription(reward affiliates.Reward, locale string) string {
	switch reward.Kind {
	case "none":
		if locale == affiliates.LocaleChinese {
			return "无奖励"
		}
		return "No reward"
	case "coupon":
		return reward.CouponName
	case "txb":
		return fmt.Sprintf("%.2f TXB", float64(reward.TXBMinor)/100)
	case "subscription_extension":
		if locale == affiliates.LocaleChinese {
			return fmt.Sprintf("%d 天", reward.ExtensionDays)
		}
		return fmt.Sprintf("%d days", reward.ExtensionDays)
	default:
		return "None"
	}
}
