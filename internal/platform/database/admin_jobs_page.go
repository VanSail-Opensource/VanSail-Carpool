package database

import (
	"context"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// ListAdminOutboxJobsPage returns a filtered stable page without inspecting payload contents.
func (s *Store) ListAdminOutboxJobsPage(ctx context.Context, cursor, search, status string, limit int) ([]model.OutboxJob, *string, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	query := outboxSelect + ` WHERE 1=1`
	args := make([]any, 0, 8)
	if search != "" {
		pattern := "%" + escapeLike(search) + "%"
		query += ` AND (id LIKE ? ESCAPE '\' COLLATE NOCASE OR kind LIKE ? ESCAPE '\' COLLATE NOCASE
			OR last_error LIKE ? ESCAPE '\' COLLATE NOCASE)`
		args = append(args, pattern, pattern, pattern)
	}
	if status != "" {
		query += ` AND status=?`
		args = append(args, status)
	}
	filter := pageFilterFingerprint(search, status)
	if cursor != "" {
		decoded, err := decodeTimestampCursor(cursor, filter)
		if err != nil {
			return nil, nil, err
		}
		query += ` AND (created_at<? OR (created_at=? AND id<?))`
		args = append(args, decoded.Timestamp, decoded.Timestamp, decoded.ID)
	}
	query += ` ORDER BY created_at DESC,id DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("list admin job page: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]model.OutboxJob, 0, limit+1)
	for rows.Next() {
		item, scanErr := scanOutbox(rows)
		if scanErr != nil {
			return nil, nil, scanErr
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
