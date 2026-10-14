package database

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// ErrInvalidCursor marks a malformed or unsupported pagination cursor.
var ErrInvalidCursor = errors.New("invalid pagination cursor")

var cursorIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type ledgerCursor struct {
	CreatedAt string `json:"t"`
	ID        string `json:"i"`
}

// ListLedgerPage returns a stable newest-first page and an opaque next cursor.
func (s *Store) ListLedgerPage(ctx context.Context, userID, cursor string, limit int) ([]model.LedgerEntry, *string, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	query := ledgerSelect + ` WHERE user_id=?`
	args := []any{userID}
	if cursor != "" {
		decoded, err := decodeLedgerCursor(cursor)
		if err != nil {
			return nil, nil, err
		}
		query += ` AND (created_at<? OR (created_at=? AND id<?))`
		args = append(args, decoded.CreatedAt, decoded.CreatedAt, decoded.ID)
	}
	query += ` ORDER BY created_at DESC,id DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("list ledger page: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]model.LedgerEntry, 0, limit+1)
	for rows.Next() {
		entry, scanErr := scanLedger(rows)
		if scanErr != nil {
			return nil, nil, scanErr
		}
		items = append(items, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if len(items) <= limit {
		return items, nil, nil
	}
	items = items[:limit]
	next, err := encodeLedgerCursor(items[len(items)-1])
	if err != nil {
		return nil, nil, err
	}
	return items, &next, nil
}

func encodeLedgerCursor(entry model.LedgerEntry) (string, error) {
	payload, err := json.Marshal(ledgerCursor{CreatedAt: stamp(entry.CreatedAt), ID: entry.ID})
	if err != nil {
		return "", fmt.Errorf("encode ledger cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeLedgerCursor(value string) (ledgerCursor, error) {
	if len(value) < 16 || len(value) > 256 {
		return ledgerCursor{}, ErrInvalidCursor
	}
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return ledgerCursor{}, ErrInvalidCursor
	}
	var cursor ledgerCursor
	if err := json.Unmarshal(payload, &cursor); err != nil || !cursorIDPattern.MatchString(cursor.ID) {
		return ledgerCursor{}, ErrInvalidCursor
	}
	parsed, err := time.Parse(time.RFC3339Nano, cursor.CreatedAt)
	if err != nil || stamp(parsed) != cursor.CreatedAt {
		return ledgerCursor{}, ErrInvalidCursor
	}
	return cursor, nil
}
