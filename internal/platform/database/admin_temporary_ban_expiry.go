package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func (s *Store) retryAttempt_QueueExpiredAdminTemporaryBan(ctx context.Context, userID string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = queueExpiredAdminTemporaryBanTx(ctx, tx, userID, now); err != nil {
		return err
	}
	return tx.Commit()
}

// Expiry continues one already-authorized restriction. It does not acquire
// fresh administrator authority from its idempotency key or request payload.
// The exact stored ban and its successful operation are locked and validated.
func queueExpiredAdminTemporaryBanTx(ctx context.Context, tx *sql.Tx, userID string, now time.Time) error {
	var actorID, banID, unbanID, expires, banStatus, operationActor, operationOwner, operationKind string
	err := tx.QueryRowContext(ctx, `SELECT b.actor_user_id,b.ban_operation_id,COALESCE(b.unban_operation_id,''),b.expires_at,
		o.status,o.actor_user_id,COALESCE(o.owner_user_id,''),o.kind
		FROM admin_temporary_bans b JOIN provider_operations o ON o.id=b.ban_operation_id
		WHERE b.user_id=$1 AND b.restored_at IS NULL FOR UPDATE OF b FOR SHARE OF o`, userID).
		Scan(&actorID, &banID, &unbanID, &expires, &banStatus, &operationActor, &operationOwner, &operationKind)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	until, err := parseStamp(expires)
	if err != nil {
		return err
	}
	if now.Before(until) || unbanID != "" {
		return nil
	}
	if banStatus != string(providerops.StatusSucceeded) || operationKind != providerops.KindAdminTemporaryBan || operationActor != actorID || operationOwner != userID {
		return ErrConflict
	}
	input, err := providerops.NormalizeCreate(providerops.CreateInput{ActorUserID: actorID, OwnerUserID: userID, Kind: providerops.KindAdminTemporaryUnban,
		IdempotencyKey: "expiry:" + banID, RequestFingerprint: "expiry:" + banID})
	if err != nil {
		return err
	}
	op, replayed, err := replayProviderOperationTx(ctx, tx, input, now)
	if err != nil {
		return err
	}
	if !replayed {
		// Only the beneficiary's current lifecycle is relevant to restoring
		// access. The original actor may since have lost administrative rights.
		if err = lockProviderOperationAccountsTx(ctx, tx, userID, userID); err != nil {
			return err
		}
		op, _, err = insertProviderOperationTx(ctx, tx, input, now, false)
		if err != nil {
			return err
		}
	}
	if op.OwnerUserID != userID {
		return ErrConflict
	}
	_, err = tx.ExecContext(ctx, `UPDATE admin_temporary_bans SET unban_operation_id=$1,unban_reason='expired',updated_at=$2 WHERE user_id=$3 AND ban_operation_id=$4`, op.Receipt.ID, stamp(now), userID, banID)
	return err
}
