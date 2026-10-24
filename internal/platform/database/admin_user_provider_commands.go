package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// AdminTemporaryBanExpiryOutboxKind queues the one-time expiry restoration check.
const AdminTemporaryBanExpiryOutboxKind = "admin_temporary_ban_expiry"

func (s *Store) retryAttempt_CreateAdminTemporaryBan(ctx context.Context, input AdminTemporaryBanInput, now time.Time) (model.OperationReceipt, error) {
	if input.DurationMinutes < 1 || input.DurationMinutes > 525600 {
		return model.OperationReceipt{}, ErrConflict
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	defer func() { _ = tx.Rollback() }()
	op, replayed, err := createProviderOperationTx(ctx, tx, providerops.CreateInput{ActorUserID: input.ActorUserID, OwnerUserID: input.UserID,
		Kind: providerops.KindAdminTemporaryBan, IdempotencyKey: input.IdempotencyKey, RequestFingerprint: input.RequestFingerprint}, now)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if replayed {
		return op.Receipt, tx.Commit()
	}
	var active string
	err = tx.QueryRowContext(ctx, `SELECT user_id FROM admin_temporary_bans WHERE user_id=? AND restored_at IS NULL`, input.UserID).Scan(&active)
	if err == nil {
		return model.OperationReceipt{}, ErrConflict
	}
	if err != sql.ErrNoRows {
		return model.OperationReceipt{}, err
	}
	expires := now.Add(time.Duration(input.DurationMinutes) * time.Minute)
	_, err = tx.ExecContext(ctx, `INSERT INTO admin_temporary_bans(user_id,actor_user_id,reason,expires_at,ban_operation_id,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, input.UserID, input.ActorUserID, input.Reason, stamp(expires), op.Receipt.ID, stamp(now), stamp(now))
	if err != nil {
		return model.OperationReceipt{}, err
	}
	payload, _ := json.Marshal(map[string]string{"userId": input.UserID})
	if err = insertOutboxTx(ctx, tx, AdminTemporaryBanExpiryOutboxKind, string(payload), expires, now); err != nil {
		return model.OperationReceipt{}, err
	}
	if err = insertAdminUserAudit(ctx, tx, input.ActorUserID, "user.temporary_ban", input.UserID, input.Reason, now); err != nil {
		return model.OperationReceipt{}, err
	}
	if err = tx.Commit(); err != nil {
		return model.OperationReceipt{}, err
	}
	return op.Receipt, nil
}

func (s *Store) retryAttempt_CreateAdminTemporaryUnban(ctx context.Context, actorID, userID, key, fingerprint, reason string, now time.Time) (model.OperationReceipt, error) {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	defer func() { _ = tx.Rollback() }()
	op, replayed, err := createProviderOperationTx(ctx, tx, providerops.CreateInput{ActorUserID: actorID, OwnerUserID: userID,
		Kind: providerops.KindAdminTemporaryUnban, IdempotencyKey: key, RequestFingerprint: fingerprint}, now)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if replayed {
		return op.Receipt, tx.Commit()
	}
	var existing, banStatus string
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(b.unban_operation_id,''),o.status FROM admin_temporary_bans b
		JOIN provider_operations o ON o.id=b.ban_operation_id WHERE b.user_id=? AND b.restored_at IS NULL`, userID).Scan(&existing, &banStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.OperationReceipt{}, ErrNotFound
		}
		return model.OperationReceipt{}, err
	}
	if existing != "" || banStatus != string(providerops.StatusSucceeded) {
		return model.OperationReceipt{}, ErrConflict
	}
	if _, err = tx.ExecContext(ctx, `UPDATE admin_temporary_bans SET unban_operation_id=?,unban_reason=?,updated_at=? WHERE user_id=?`, op.Receipt.ID, reason, stamp(now), userID); err != nil {
		return model.OperationReceipt{}, err
	}
	if err = insertAdminUserAudit(ctx, tx, actorID, "user.temporary_unban", userID, reason, now); err != nil {
		return model.OperationReceipt{}, err
	}
	if err = tx.Commit(); err != nil {
		return model.OperationReceipt{}, err
	}
	return op.Receipt, nil
}

func (s *Store) retryAttempt_QueueExpiredAdminTemporaryBan(ctx context.Context, userID string, now time.Time) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var actorID, unbanID, banStatus string
	err = tx.QueryRowContext(ctx, `SELECT b.actor_user_id,COALESCE(b.unban_operation_id,''),o.status FROM admin_temporary_bans b
		JOIN provider_operations o ON o.id=b.ban_operation_id WHERE b.user_id=? AND b.restored_at IS NULL AND b.expires_at<=?`, userID, stamp(now)).Scan(&actorID, &unbanID, &banStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	if unbanID != "" {
		return tx.Commit()
	}
	if banStatus != string(providerops.StatusSucceeded) {
		return ErrConflict
	}
	op, _, err := createProviderOperationTx(ctx, tx, providerops.CreateInput{ActorUserID: actorID, OwnerUserID: userID, Kind: providerops.KindAdminTemporaryUnban,
		IdempotencyKey: "expiry:" + userID, RequestFingerprint: "expiry:" + stamp(now.Truncate(time.Minute))}, now)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE admin_temporary_bans SET unban_operation_id=?,unban_reason='expired',updated_at=? WHERE user_id=?`, op.Receipt.ID, stamp(now), userID)
	if err != nil {
		return err
	}
	return tx.Commit()
}
