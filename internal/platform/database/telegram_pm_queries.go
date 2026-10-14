package database

import (
	"context"
	"database/sql"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const pmConversationSelect = `SELECT c.id,c.user_id,c.chat_id,COALESCE(c.topic_id,0),COALESCE(c.profile_message_id,0),
	c.topic_state,COALESCE(c.topic_operation_id,''),c.profile_state,c.created_at,c.updated_at,u.telegram_id,u.telegram_first_name,
	u.telegram_last_name,u.telegram_username,u.notification_locale,u.pm_blocked,u.pm_muted
	FROM telegram_pm_conversations c JOIN users u ON u.id=c.user_id`

func scanPMConversation(row rowScanner) (model.PMConversation, error) {
	var item model.PMConversation
	var created, updated string
	err := row.Scan(&item.ID, &item.UserID, &item.ChatID, &item.TopicID, &item.ProfileMessageID, &item.TopicState,
		&item.TopicOperationID, &item.ProfileState, &created, &updated, &item.TelegramID, &item.FirstName, &item.LastName, &item.Username, &item.Locale, &item.Blocked, &item.Muted)
	if errors.Is(err, sql.ErrNoRows) {
		return item, ErrNotFound
	}
	if err != nil {
		return item, err
	}
	item.CreatedAt, err = parseStamp(created)
	if err == nil {
		item.UpdatedAt, err = parseStamp(updated)
	}
	return item, err
}

func (s *Store) PMConversation(ctx context.Context, id string) (model.PMConversation, error) {
	return scanPMConversation(s.db.QueryRowContext(ctx, pmConversationSelect+` WHERE c.id=?`, id))
}

// PMConversationByUser returns the user's most recently used PM route, if any.
func (s *Store) PMConversationByUser(ctx context.Context, userID string) (model.PMConversation, bool, error) {
	item, err := scanPMConversation(s.db.QueryRowContext(ctx, pmConversationSelect+` WHERE c.user_id=? ORDER BY c.updated_at DESC,c.id DESC LIMIT 1`, userID))
	if errors.Is(err, ErrNotFound) {
		return item, false, nil
	}
	return item, err == nil, err
}

func (s *Store) PMConversationByTopic(ctx context.Context, chatID, topicID int64) (model.PMConversation, bool, error) {
	item, err := scanPMConversation(s.db.QueryRowContext(ctx, pmConversationSelect+` WHERE c.chat_id=? AND c.topic_id=?`, chatID, topicID))
	if errors.Is(err, ErrNotFound) {
		return item, false, nil
	}
	return item, err == nil, err
}

func (s *Store) PMUserFlags(ctx context.Context, userID string) (blocked, muted bool, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT pm_blocked,pm_muted FROM users WHERE id=?`, userID).Scan(&blocked, &muted)
	return
}
