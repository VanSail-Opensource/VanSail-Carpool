package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// HasProvisionablePurchase limits remote creation to paid, unexpired access.
func (s *Store) HasProvisionablePurchase(ctx context.Context, userID string, now time.Time) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchases WHERE user_id=?
		AND status IN ('active','activating','queued') AND valid_until>?)`, userID, stamp(now)).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check provisionable purchase: %w", err)
	}
	return exists == 1, nil
}

// LinkProvisionedRemnaUser changes the provider ID only if local identity is stable.
func (s *Store) LinkProvisionedRemnaUser(ctx context.Context, userID, username, previousID, remoteID string, now time.Time) error {
	if userID == "" || username == "" || remoteID == "" {
		return ErrConflict
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	result, err := s.db.ExecContext(ctx, `UPDATE users SET remna_user_id=?,remna_subscription_url=NULL,recovery_reason='',updated_at=?
		WHERE id=? AND username=? AND onboarding_state='complete' AND COALESCE(remna_user_id,'')=?
		AND EXISTS(SELECT 1 FROM purchases WHERE user_id=? AND status IN ('active','activating','queued') AND valid_until>?)`,
		remoteID, stamp(now), userID, username, previousID, userID, stamp(now))
	if err != nil {
		if isUniqueConstraint(err) {
			return ErrConflict
		}
		return fmt.Errorf("link provisioned Remnawave user: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected != 1 {
		return ErrConflict
	}
	return nil
}

type conflictedPurchase struct {
	id, name string
	charged  int64
}

// ResolveProvisioningConflict refunds access that cannot use its reserved name.
func (s *Store) ResolveProvisioningConflict(ctx context.Context, userID, username string, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin provisioning conflict refund: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var currentUsername, state, reason sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT username,onboarding_state,recovery_reason FROM users WHERE id=?`, userID).
		Scan(&currentUsername, &state, &reason); err != nil {
		return err
	}
	if state.String == "username" && reason.String == "remnawave_username_conflict" {
		return nil
	}
	if state.String != "complete" || currentUsername.String != username {
		return ErrConflict
	}
	rows, err := tx.QueryContext(ctx, `SELECT purchases.id,combos.name,purchases.charged_txb_minor FROM purchases
		JOIN combos ON combos.id=purchases.combo_id WHERE purchases.user_id=?
		AND purchases.status IN ('active','activating','queued') AND purchases.valid_until>?
		ORDER BY purchases.valid_from,purchases.id`, userID, stamp(now))
	if err != nil {
		return err
	}
	purchases := make([]conflictedPurchase, 0)
	for rows.Next() {
		var purchase conflictedPurchase
		if err := rows.Scan(&purchase.id, &purchase.name, &purchase.charged); err != nil {
			_ = rows.Close()
			return err
		}
		purchases = append(purchases, purchase)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()
	var refunded, balance int64
	names := make([]string, 0, len(purchases))
	for _, purchase := range purchases {
		result, err := tx.ExecContext(ctx, `UPDATE purchases SET status='cancelled',auto_renew_enabled=0,updated_at=? WHERE id=?
			AND status IN ('active','activating','queued')`, stamp(now), purchase.id)
		if err != nil {
			return err
		}
		if affected, err := result.RowsAffected(); err != nil || affected != 1 {
			if err != nil {
				return err
			}
			return ErrConflict
		}
		balance, err = changeBalanceTx(ctx, tx, userID, purchase.charged, now)
		if err != nil {
			return err
		}
		if _, err := insertLedgerTx(ctx, tx, userID, purchase.charged, balance, "remna_identity_conflict_refund",
			purchase.id, "Remnawave username belongs to another Telegram user", now); err != nil {
			return err
		}
		if refunded > 1<<63-1-purchase.charged {
			return errors.New("provisioning refund exceeds supported amount")
		}
		refunded += purchase.charged
		names = append(names, purchase.name)
		if _, err := tx.ExecContext(ctx, `DELETE FROM outbox_jobs WHERE status IN ('pending','failed')
			AND kind IN ('remna_apply_entitlement','remna_prepare_continuity','rollover_finalize')
			AND json_extract(payload,'$.purchaseId')=?`, purchase.id); err != nil {
			return err
		}
	}
	// A previously onboarded identity must not repeat first-entry verification
	// when a confirmed username collision restarts account setup.
	if _, err := tx.ExecContext(ctx, `INSERT INTO panel_entry_verification(telegram_id,checked)
		SELECT telegram_id,1 FROM users WHERE id=? AND policy_accepted_at IS NOT NULL
		ON CONFLICT(telegram_id) DO UPDATE SET checked=1`, userID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE users SET username=NULL,onboarding_state='username',policy_accepted_at=NULL,
		accepted_agreement_revision=0,remna_user_id=NULL,remna_subscription_url=NULL,
		recovery_reason='remnawave_username_conflict',updated_at=? WHERE id=? AND username=? AND onboarding_state='complete'`,
		stamp(now), userID, username)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return err
		}
		return ErrConflict
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_notification_events WHERE user_id=? AND queued_at IS NULL AND gate_key=?`,
		userID, userSyncGate(userID)); err != nil {
		return err
	}
	if len(purchases) > 0 {
		key, err := ids.New()
		if err != nil {
			return err
		}
		if _, err := s.insertUserNotificationTx(ctx, tx, "identity-conflict:"+key, userID,
			jobpayload.UserEventProvisionConflict, "", map[string]string{
				notifications.FactCancelledCombos: strings.Join(names, ", "),
				notifications.FactAmount:          fmt.Sprint(refunded),
				notifications.FactBalance:         fmt.Sprint(balance),
				notifications.FactTime:            stamp(now),
			}, now); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit provisioning conflict refund: %w", err)
	}
	return nil
}
