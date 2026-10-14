package database

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (s *Store) ListPMConversations(ctx context.Context, cursor, search string, limit int) ([]model.PMConversation, *string, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	query, args := pmConversationSelect+` WHERE 1=1`, []any{}
	if search != "" {
		pattern := "%" + escapeLike(search) + "%"
		query += ` AND ((u.telegram_first_name||' '||u.telegram_last_name) LIKE ? ESCAPE '\' COLLATE NOCASE OR u.telegram_username LIKE ? ESCAPE '\' COLLATE NOCASE OR CAST(u.telegram_id AS TEXT) LIKE ? ESCAPE '\')`
		args = append(args, pattern, pattern, pattern)
	}
	filter := pageFilterFingerprint(search)
	if cursor != "" {
		decoded, err := decodeTimestampCursor(cursor, filter)
		if err != nil {
			return nil, nil, err
		}
		query += ` AND (c.created_at<? OR (c.created_at=? AND c.id<?))`
		args = append(args, decoded.Timestamp, decoded.Timestamp, decoded.ID)
	}
	query += ` ORDER BY c.created_at DESC,c.id DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]model.PMConversation, 0, limit+1)
	for rows.Next() {
		item, err := scanPMConversation(rows)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if len(items) <= limit {
		return items, nil, nil
	}
	items = items[:limit]
	last := items[len(items)-1]
	next, err := encodeTimestampCursor(last.CreatedAt, last.ID, filter)
	return items, &next, err
}

func (s *Store) ListPMDeliveries(ctx context.Context, id string) ([]model.PMDelivery, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT o.id,o.status,o.error_code,o.created_at,r.target_type,r.target_id,r.provider_reference,p.provider_reference,
		read_state.read_at,COALESCE(read_state.source,'')
		FROM provider_operations o JOIN provider_operation_items p ON p.operation_id=o.id AND p.item_key='topic' AND p.target_id=?
		JOIN provider_operation_items r ON r.operation_id=o.id AND r.item_key='relay'
		LEFT JOIN telegram_pm_read_status read_state ON read_state.operation_id=o.id WHERE o.kind='telegram_pm_relay'
		ORDER BY o.created_at DESC,o.id DESC LIMIT 50`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]model.PMDelivery, 0)
	for rows.Next() {
		var item model.PMDelivery
		var created, targetType, targetID, reference, topic string
		var readAt sql.NullString
		if err := rows.Scan(&item.OperationID, &item.Status, &item.ErrorCode, &created, &targetType, &targetID, &reference, &topic, &readAt, &item.ReadSource); err != nil {
			return nil, err
		}
		item.CreatedAt, err = parseStamp(created)
		if err != nil {
			return nil, err
		}
		item.Direction = "outbound"
		if targetType == "pm_inbound_message" {
			item.Direction = "inbound"
		}
		chat, message, ok := strings.Cut(targetID, ":")
		if !ok {
			return nil, fmt.Errorf("invalid PM source reference")
		}
		item.SourceChatID, err = strconv.ParseInt(chat, 10, 64)
		if err != nil {
			return nil, err
		}
		item.SourceMessageID, err = strconv.ParseInt(message, 10, 64)
		if err != nil {
			return nil, err
		}
		if reference != "" {
			item.ResultMessageID, err = strconv.ParseInt(reference, 10, 64)
			if err != nil {
				return nil, err
			}
		}
		if topic != "" {
			item.TopicID, err = strconv.ParseInt(topic, 10, 64)
			if err != nil {
				return nil, err
			}
		}
		if readAt.Valid {
			read, parseErr := parseStamp(readAt.String)
			if parseErr != nil {
				return nil, parseErr
			}
			item.ReadAt = &read
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
