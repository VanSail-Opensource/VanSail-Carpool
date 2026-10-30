package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/affiliates"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

func settleAffiliateTx(ctx context.Context, tx *sql.Tx, orderID, invitedUserID string, amount int64, now time.Time) error {
	var inviterID, inviteeName, locale string
	var telegramID int64
	err := tx.QueryRowContext(ctx, `SELECT inviter.id,inviter.telegram_id,
		COALESCE(NULLIF(invitee.telegram_username,''),NULLIF(invitee.telegram_first_name,''),''),inviter.notification_locale
		FROM users invitee JOIN users inviter ON inviter.telegram_id=invitee.inviter_id WHERE invitee.id=? AND inviter.account_status='active'`, invitedUserID).
		Scan(&inviterID, &telegramID, &inviteeName, &locale)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var prior int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM payment_orders WHERE user_id=? AND id<>? AND paid_at IS NOT NULL`, invitedUserID, orderID).Scan(&prior); err != nil {
		return err
	}
	if prior > 0 {
		return nil
	}
	config, err := affiliateConfig(ctx, tx)
	if err != nil {
		return err
	}
	var successCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM affiliate_settlements WHERE inviter_user_id=?`, inviterID).Scan(&successCount); err != nil {
		return err
	}
	current := enabledTierAt(config.Tiers, successCount)
	bps := 0
	if current.CommissionEnabled {
		bps = current.CommissionBPS
	}
	commission := floorBasisPoints(amount, bps)
	settlementID, err := ids.New()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO affiliate_settlements(id,invited_user_id,inviter_user_id,payment_order_id,config_id,tier_id,tier_name,
		commission_bps,topup_txb_minor,commission_txb_minor,settled_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, settlementID, invitedUserID, inviterID,
		orderID, config.ID, current.ID, current.Name, bps, amount, commission, stamp(now))
	if err != nil {
		if isUniqueConstraint(err) {
			return nil
		}
		return err
	}
	if commission > 0 {
		balance, creditErr := adjustBalanceTx(ctx, tx, inviterID, commission, now)
		if creditErr != nil {
			return creditErr
		}
		if _, err := insertLedgerTx(ctx, tx, inviterID, commission, balance, "affiliate_commission", settlementID, inviteeName, now); err != nil {
			return err
		}
	}
	success := jobpayload.AffiliateSuccess{SettlementID: settlementID, ChatID: telegramID, Locale: locale, InviteeName: inviteeName,
		SettledAt: stamp(now), CommissionMinor: commission, TierName: current.Name}
	if err := insertAffiliatePayloadTx(ctx, tx, jobpayload.AffiliateSuccessKind, success, now); err != nil {
		return err
	}
	return awardCrossedAffiliateTierTx(ctx, tx, config, inviterID, telegramID, settlementID, successCount+1, locale, now)
}

func enabledTierAt(tiers []affiliates.Tier, successful int) affiliates.Tier {
	var current affiliates.Tier
	for _, tier := range tiers {
		if tier.Enabled && tier.Threshold <= successful {
			current = tier
		}
	}
	return current
}

func floorBasisPoints(amount int64, bps int) int64 {
	return (amount/10000)*int64(bps) + (amount%10000)*int64(bps)/10000
}

func insertAffiliatePayloadTx(ctx context.Context, tx *sql.Tx, kind string, payload any, now time.Time) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return insertOutboxTx(ctx, tx, kind, string(encoded), now, now)
}
