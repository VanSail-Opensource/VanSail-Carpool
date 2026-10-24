package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func pmFingerprint(input any) (string, error) {
	payload, err := json.Marshal(input)
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), err
}

func recordPMUpdateTx(ctx context.Context, tx *sql.Tx, updateID int64, now time.Time) (bool, error) {
	if updateID <= 0 {
		return false, ErrConflict
	}
	result, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO telegram_pm_updates(update_id,created_at) VALUES(?,?)`, updateID, stamp(now))
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func (s *Store) retryAttempt_QueuePMNotice(ctx context.Context, updateID int64, notice model.PMNotice, now time.Time) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	added, err := recordPMUpdateTx(ctx, tx, updateID, now)
	if err != nil || !added {
		return err
	}
	payload, err := json.Marshal(notice)
	if err != nil {
		return err
	}
	if err := insertOutboxTx(ctx, tx, "telegram_pm_notice", string(payload), now, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) retryAttempt_QueuePMRelay(ctx context.Context, input model.PMRelayInput, now time.Time) (*model.OperationReceipt, error) {
	if input.ChatID >= 0 || input.SourceChatID == 0 || input.SourceMessageID <= 0 {
		return nil, ErrConflict
	}
	if input.Inbound && input.ActorUserID != input.UserID {
		return nil, ErrConflict
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	added, err := recordPMUpdateTx(ctx, tx, input.UpdateID, now)
	if err != nil || !added {
		return nil, err
	}
	if !input.Inbound {
		if err := requirePMAdminTx(ctx, tx, input.ActorUserID); err != nil {
			return nil, err
		}
	}
	var eligible, blocked bool
	if err := tx.QueryRowContext(ctx, `SELECT (policy_accepted_at IS NOT NULL OR accepted_agreement_revision>0 OR EXISTS(
		SELECT 1 FROM panel_entry_verification v WHERE v.telegram_id=users.telegram_id AND v.checked=1)),pm_blocked FROM users WHERE id=?`, input.UserID).Scan(&eligible, &blocked); err != nil {
		return nil, err
	}
	if !eligible || input.Inbound && blocked {
		return nil, ErrConflict
	}
	id, err := ids.New()
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO telegram_pm_conversations(id,user_id,chat_id,created_at,updated_at) VALUES(?,?,?,?,?)
		ON CONFLICT(user_id,chat_id) DO UPDATE SET updated_at=excluded.updated_at`, id, input.UserID, input.ChatID, stamp(now), stamp(now)); err != nil {
		return nil, err
	}
	conversation, err := scanPMConversation(tx.QueryRowContext(ctx, pmConversationSelect+` WHERE c.user_id=? AND c.chat_id=?`, input.UserID, input.ChatID))
	if err != nil {
		return nil, err
	}
	if input.Inbound && input.SourceChatID != conversation.TelegramID || !input.Inbound && input.SourceChatID != conversation.ChatID {
		return nil, ErrConflict
	}
	if input.Inbound {
		occurredAt := input.MessageAt.UTC()
		if input.MessageAt.IsZero() {
			occurredAt = now.UTC()
		}
		if err := markPriorPMDeliveriesReadTx(ctx, tx, conversation.ID, input.ReplyToMessageID, occurredAt); err != nil {
			return nil, err
		}
	}
	targetType := "pm_outbound_message"
	if input.Inbound {
		targetType = "pm_inbound_message"
	}
	ref := strconv.FormatInt(input.SourceChatID, 10) + ":" + strconv.FormatInt(input.SourceMessageID, 10)
	fingerprint, err := pmFingerprint(input)
	if err != nil {
		return nil, err
	}
	items := []providerops.ItemInput{{Key: "topic", TargetType: "pm_conversation", TargetID: conversation.ID}, {Key: "profile", TargetType: "pm_conversation", TargetID: conversation.ID}, {Key: "relay", TargetType: targetType, TargetID: ref}}
	if !input.Inbound && (input.AttributionReply || len(input.Content) == 0) {
		items = append(items, providerops.ItemInput{Key: "footer", TargetType: "pm_outbound_footer", TargetID: ref})
	}
	operation, _, err := createProviderOperationTx(ctx, tx, providerops.CreateInput{ActorUserID: input.ActorUserID, OwnerUserID: input.UserID,
		Kind: providerops.KindTelegramPMRelay, IdempotencyKey: "update:" + strconv.FormatInt(input.UpdateID, 10), RequestFingerprint: fingerprint,
		Items: items}, now)
	if err != nil {
		return nil, err
	}
	if err := s.savePMContentTx(ctx, tx, operation.Receipt.ID, input, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &operation.Receipt, nil
}

func requirePMAdminTx(ctx context.Context, tx *sql.Tx, actorID string) error {
	var authorized bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=? AND role='admin')`, actorID).Scan(&authorized); err != nil {
		return err
	}
	if !authorized {
		return ErrConflict
	}
	return nil
}
