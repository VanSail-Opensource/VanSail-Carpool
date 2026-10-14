package database

import (
	"context"
	"database/sql"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
)

func (s *Store) MemberRecords(ctx context.Context, userID, cursor string, limit int) (abuse.RecordPage, error) {
	where, args, err := abuseRecordWhere(` WHERE record.user_id=? AND record.deleted_at IS NULL`, []any{userID}, cursor, `member:`+userID)
	if err != nil {
		return abuse.RecordPage{}, err
	}
	return s.abuseRecordPage(ctx, where, args, limit, `member:`+userID)
}
func (s *Store) AdminAbuseRecords(ctx context.Context, cursor string, limit int) (abuse.RecordPage, error) {
	where, args, err := abuseRecordWhere(` WHERE record.deleted_at IS NULL`, nil, cursor, `admin`)
	if err != nil {
		return abuse.RecordPage{}, err
	}
	return s.abuseRecordPage(ctx, where, args, limit, `admin`)
}
func abuseRecordWhere(where string, args []any, cursor, filter string) (string, []any, error) {
	if cursor == "" {
		return where, args, nil
	}
	decoded, err := decodeTimestampCursor(cursor, filter)
	if err == nil {
		return where + ` AND (record.created_at<? OR (record.created_at=? AND record.id<?))`, append(args, decoded.Timestamp, decoded.Timestamp, decoded.ID), nil
	}
	legacy, legacyErr := parseStamp(cursor)
	if legacyErr != nil {
		return ``, nil, ErrInvalidCursor
	}
	return where + ` AND record.created_at<?`, append(args, stamp(legacy)), nil
}
func (s *Store) abuseRecordPage(ctx context.Context, where string, args []any, limit int, filter string) (abuse.RecordPage, error) {
	if limit < 1 || limit > 100 {
		return abuse.RecordPage{}, abuse.ErrInvalid
	}
	args = append(args, limit+1)
	query := `SELECT record.id,COALESCE(user.username,''),record.created_at,record.measured_qps,record.qps_limit,record.selected_action,record.expires_at,COALESCE(GROUP_CONCAT(reason.name,', '),'global') FROM abuse_records record JOIN users user ON user.id=record.user_id LEFT JOIN abuse_record_reasons reason ON reason.record_id=record.id` + where + ` GROUP BY record.id ORDER BY record.created_at DESC,record.id DESC LIMIT ?`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return abuse.RecordPage{}, err
	}
	defer rows.Close()
	page := abuse.RecordPage{Items: []abuse.Record{}}
	for rows.Next() {
		var item abuse.Record
		var created string
		var expires sql.NullString
		if err = rows.Scan(&item.ID, &item.Username, &created, &item.MeasuredQPS, &item.QPSLimit, &item.Action, &expires, &item.Reason); err != nil {
			return page, err
		}
		item.OccurredAt, err = parseStamp(created)
		if err != nil {
			return page, err
		}
		if expires.Valid {
			parsed, parseErr := parseStamp(expires.String)
			if parseErr != nil {
				return page, parseErr
			}
			item.ExpiresAt = &parsed
		}
		page.Items = append(page.Items, item)
	}
	if len(page.Items) > limit {
		page.NextCursor, err = encodeTimestampCursor(page.Items[limit].OccurredAt, page.Items[limit].ID, filter)
		if err != nil {
			return page, err
		}
		page.Items = page.Items[:limit]
	}
	return page, rows.Err()
}
