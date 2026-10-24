package database

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// QueuePMModeration commits normalized user flags and a recoverable profile update.
func (s *Store) retryAttempt_QueuePMModeration(ctx context.Context, actorID, key string, input model.PMModerationInput, updateID int64, now time.Time) (model.OperationReceipt, error) {
	if input.Blocked == nil && input.Muted == nil {
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
	if err := requirePMAdminTx(ctx, tx, actorID); err != nil {
		return model.OperationReceipt{}, err
	}
	conversation, err := scanPMConversation(tx.QueryRowContext(ctx, pmConversationSelect+` WHERE c.id=?`, input.ConversationID))
	if err != nil {
		return model.OperationReceipt{}, err
	}
	fingerprint, err := pmFingerprint(input)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	operation, replayed, err := createProviderOperationTx(ctx, tx, providerops.CreateInput{ActorUserID: actorID, OwnerUserID: conversation.UserID,
		Kind: providerops.KindTelegramPMProfile, IdempotencyKey: key, RequestFingerprint: fingerprint,
		Items: []providerops.ItemInput{{Key: "topic", TargetType: "pm_conversation", TargetID: conversation.ID}, {Key: "profile", TargetType: "pm_conversation", TargetID: conversation.ID}}}, now)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if !replayed {
		if updateID > 0 {
			added, err := recordPMUpdateTx(ctx, tx, updateID, now)
			if err != nil {
				return model.OperationReceipt{}, err
			}
			if !added {
				return model.OperationReceipt{}, ErrConflict
			}
		}
		blocked, muted := conversation.Blocked, conversation.Muted
		if input.Blocked != nil {
			blocked = *input.Blocked
		}
		if input.Muted != nil {
			muted = *input.Muted
		}
		if _, err := tx.ExecContext(ctx, `UPDATE users SET pm_blocked=?,pm_muted=?,updated_at=? WHERE id=?`, blocked, muted, stamp(now), conversation.UserID); err != nil {
			return model.OperationReceipt{}, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE telegram_pm_conversations SET updated_at=? WHERE user_id=?`, stamp(now), conversation.UserID); err != nil {
			return model.OperationReceipt{}, err
		}
		auditID, err := ids.New()
		if err != nil {
			return model.OperationReceipt{}, err
		}
		if err := insertAuditTx(ctx, tx, auditID, &actorID, "telegram_pm.moderation", "user", conversation.UserID, `{"blocked":`+strconv.FormatBool(blocked)+`,"muted":`+strconv.FormatBool(muted)+`}`, now); err != nil {
			return model.OperationReceipt{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.OperationReceipt{}, err
	}
	return operation.Receipt, nil
}

// QueuePMProfileRefresh records an authorized profile-card refresh request.
func (s *Store) retryAttempt_QueuePMProfileRefresh(ctx context.Context, actorID, key, conversationID string, updateID int64, now time.Time) (model.OperationReceipt, error) {
	if strings.TrimSpace(key) == "" || strings.TrimSpace(conversationID) == "" {
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
	if err := requirePMAdminTx(ctx, tx, actorID); err != nil {
		return model.OperationReceipt{}, err
	}
	conversation, err := scanPMConversation(tx.QueryRowContext(ctx, pmConversationSelect+` WHERE c.id=?`, conversationID))
	if err != nil {
		return model.OperationReceipt{}, err
	}
	fingerprint, err := pmFingerprint(struct {
		ConversationID string
		Action         string
	}{conversationID, "refresh"})
	if err != nil {
		return model.OperationReceipt{}, err
	}
	operation, replayed, err := createProviderOperationTx(ctx, tx, providerops.CreateInput{
		ActorUserID: actorID, OwnerUserID: conversation.UserID, Kind: providerops.KindTelegramPMProfile,
		IdempotencyKey: key, RequestFingerprint: fingerprint,
		Items: []providerops.ItemInput{{Key: "topic", TargetType: "pm_conversation", TargetID: conversation.ID}, {Key: "profile", TargetType: "pm_conversation", TargetID: conversation.ID}},
	}, now)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if !replayed && updateID > 0 {
		added, err := recordPMUpdateTx(ctx, tx, updateID, now)
		if err != nil {
			return model.OperationReceipt{}, err
		}
		if !added {
			return model.OperationReceipt{}, ErrConflict
		}
	}
	if err := tx.Commit(); err != nil {
		return model.OperationReceipt{}, err
	}
	return operation.Receipt, nil
}

func (s *Store) retryAttempt_QueuePMTopicRepair(ctx context.Context, actorID, key string, input model.PMTopicRepairInput, now time.Time) (model.OperationReceipt, error) {
	if input.TopicID <= 1 || input.ProfileMessageID < 0 {
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
	if err := requirePMAdminTx(ctx, tx, actorID); err != nil {
		return model.OperationReceipt{}, err
	}
	conversation, err := scanPMConversation(tx.QueryRowContext(ctx, pmConversationSelect+` WHERE c.id=?`, input.ConversationID))
	if err != nil {
		return model.OperationReceipt{}, err
	}
	fingerprint, err := pmFingerprint(input)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	operation, replayed, err := createProviderOperationTx(ctx, tx, providerops.CreateInput{ActorUserID: actorID, OwnerUserID: conversation.UserID,
		Kind: providerops.KindTelegramPMRepair, IdempotencyKey: key, RequestFingerprint: fingerprint,
		Items: []providerops.ItemInput{{Key: "repair", TargetType: "pm_topic_recovery", TargetID: conversation.ID + ":" + strconv.FormatInt(input.TopicID, 10) + ":" + strconv.FormatInt(input.ProfileMessageID, 10)}}}, now)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if !replayed && conversation.TopicState != "pending_review" && conversation.ProfileState != "pending_review" {
		return model.OperationReceipt{}, ErrConflict
	}
	if !replayed {
		var conflict bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM telegram_pm_conversations WHERE chat_id=? AND topic_id=? AND id<>?)
			OR EXISTS(SELECT 1 FROM provider_operation_items WHERE status='processing' AND
			((target_type='pm_conversation' AND target_id=? AND item_key='profile') OR (target_type='pm_topic_recovery' AND target_id LIKE ?)))
			OR EXISTS(SELECT 1 FROM provider_operation_items item JOIN provider_operations operation ON operation.id=item.operation_id
			WHERE item.target_type='pm_topic_recovery' AND item.target_id LIKE ? AND operation.id<>? AND operation.status IN ('queued','processing'))
			OR EXISTS(SELECT 1 FROM provider_operation_items item JOIN provider_operations operation ON operation.id=item.operation_id
			JOIN telegram_pm_conversations other ON other.id=substr(item.target_id,1,instr(item.target_id,':')-1)
			WHERE item.target_type='pm_topic_recovery' AND other.chat_id=? AND other.id<>? AND item.target_id LIKE other.id||':'||?||':%'
			AND operation.status IN ('queued','processing','pending_review'))`, conversation.ChatID, input.TopicID, conversation.ID, conversation.ID, conversation.ID+":%", conversation.ID+":%", operation.Receipt.ID, conversation.ChatID, conversation.ID, strconv.FormatInt(input.TopicID, 10)).Scan(&conflict); err != nil {
			return model.OperationReceipt{}, err
		}
		if conflict || conversation.TopicState == "creating" {
			return model.OperationReceipt{}, ErrConflict
		}
		if err := insertAdminUserAudit(ctx, tx, actorID, "telegram_pm.topic_repair", conversation.UserID, "topic="+strconv.FormatInt(input.TopicID, 10)+", profile="+strconv.FormatInt(input.ProfileMessageID, 10), now); err != nil {
			return model.OperationReceipt{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.OperationReceipt{}, err
	}
	return operation.Receipt, nil
}
