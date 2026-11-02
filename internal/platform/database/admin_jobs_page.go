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
		query += fmt.Sprintf(" AND (id ILIKE $%d ESCAPE '\\'  OR kind ILIKE $%d ESCAPE '\\' \n\t\t\tOR last_error ILIKE $%d ESCAPE '\\' )", len(args)+1, len(args)+2, len(args)+3)

		args = append(args, pattern, pattern, pattern)
	}
	if status != "" {
		query += fmt.Sprintf(" AND status=$%d", len(args)+1)

		args = append(args, status)
	}
	filter := pageFilterFingerprint(search, status)
	if cursor != "" {
		decoded, err := decodeTimestampCursor(cursor, filter)
		if err != nil {
			return nil, nil, err
		}
		query += fmt.Sprintf(" AND (created_at<$%d OR (created_at=$%d AND id<$%d))", len(args)+1, len(args)+2, len(args)+3)

		args = append(args, decoded.Timestamp, decoded.Timestamp, decoded.ID)
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC,id DESC LIMIT $%d", len(args)+1)

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
